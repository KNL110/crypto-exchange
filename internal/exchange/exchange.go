package exchange

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
)

// Exchange coordinates trading across all configured symbols.
//
// Users are wallet addresses: every order and cancel must be signed by the
// maker (see signing.go). privateKey is the exchange's own operator key, used
// to submit settlement transactions — never a user's key.
type Exchange struct {
	privateKey  *ecdsa.PrivateKey
	chainID     *big.Int
	orderBooks  map[Symbol]*SymbolBook
	nextOrderID atomic.Uint64

	// usedNonces stops a signed order from being submitted twice.
	// TODO: persist this; it is lost on restart.
	noncesMu   sync.Mutex
	usedNonces map[common.Address]map[uint64]struct{}
}

// NewExchange creates an Exchange with an empty order book for each symbol.
// chainID is part of the EIP-712 domain, so signatures from other chains are rejected.
func NewExchange(privateKey string, chainID *big.Int) (*Exchange, error) {
	books := make(map[Symbol]*SymbolBook)
	for _, sym := range symbols {
		books[sym] = newSymbolBook(sym)
	}
	prvKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key(Exchange): %w", err)
	}
	return &Exchange{
		orderBooks: books,
		privateKey: prvKey,
		chainID:    chainID,
		usedNonces: make(map[common.Address]map[uint64]struct{}),
	}, nil
}

func (e *Exchange) symbolBook(symbol Symbol) (*SymbolBook, error) {
	sb, ok := e.orderBooks[symbol]
	if !ok {
		return nil, ErrUnknownSymbol
	}
	return sb, nil
}

// Symbols returns the configured trading symbols.
func (e *Exchange) Symbols() []Symbol {
	symbols := make([]Symbol, 0, len(e.orderBooks))
	for s := range e.orderBooks {
		symbols = append(symbols, s)
	}
	return symbols
}

// authorizeOrder checks the maker signed msg, that it has not expired, and
// reserves its nonce. Call releaseNonce if the order is then not accepted.
func (e *Exchange) authorizeOrder(msg OrderMessage, sig []byte) error {
	signer, err := recoverSigner(msg.typedData(e.chainID), sig)
	if err != nil {
		return err
	}
	if signer != msg.Maker {
		return ErrInvalidSignature
	}
	if msg.Expiry <= uint64(time.Now().Unix()) {
		return ErrOrderExpired
	}

	e.noncesMu.Lock()
	defer e.noncesMu.Unlock()
	used, ok := e.usedNonces[msg.Maker]
	if !ok {
		used = make(map[uint64]struct{})
		e.usedNonces[msg.Maker] = used
	}
	if _, dup := used[msg.Nonce]; dup {
		return ErrNonceUsed
	}
	used[msg.Nonce] = struct{}{}
	return nil
}

func (e *Exchange) releaseNonce(maker common.Address, nonce uint64) {
	e.noncesMu.Lock()
	defer e.noncesMu.Unlock()
	delete(e.usedNonces[maker], nonce)
}

// TODO:consider the performance impact of copying the order struct in all these services
// placeLimitOrder trades the order against the book as far as its limit price
// allows, then rests whatever is left. It returns the fills (possibly none)
// and a snapshot of the order.
func (e *Exchange) placeLimitOrder(msg OrderMessage, sig []byte) ([]matchengine.MatchedOrder, *matchengine.Order, error) {
	if msg.IsMarket {
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", ErrOrderTypeMismatch)
	}
	if msg.Size == 0 {
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", ErrInvalidSize)
	}
	if msg.Price == 0 {
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", ErrInvalidPrice)
	}

	sb, err := e.symbolBook(msg.Symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", err)
	}
	if err := e.authorizeOrder(msg, sig); err != nil {
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", err)
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	id := e.nextOrderID.Add(1)
	order := matchengine.NewOrder(id, msg.IsBid, msg.Size, msg.Maker)
	order.Expiry = msg.Expiry

	matched, _, err := sb.book.PlaceLimitOrder(order, msg.Price)
	if err != nil {
		e.releaseNonce(msg.Maker, msg.Nonce)
		return nil, nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w", err)
	}
	snap := *order
	return matched, &snap, nil
}

func (e *Exchange) placeMarketOrder(msg OrderMessage, sig []byte) ([]matchengine.MatchedOrder, *matchengine.Order, error) {
	if !msg.IsMarket {
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", ErrOrderTypeMismatch)
	}
	if msg.Size == 0 {
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", ErrInvalidSize)
	}
	// Price is the worst price the maker accepts; without one a market
	// order could fill at any price.
	if msg.Price == 0 {
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", ErrInvalidPrice)
	}

	sb, err := e.symbolBook(msg.Symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", err)
	}
	if err := e.authorizeOrder(msg, sig); err != nil {
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", err)
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	id := e.nextOrderID.Add(1)
	order := matchengine.NewOrder(id, msg.IsBid, msg.Size, msg.Maker)

	matched, _, err := sb.book.PlaceMarketOrder(order, msg.Price)
	if err != nil {
		e.releaseNonce(msg.Maker, msg.Nonce)
		if errors.Is(err, matchengine.ErrInsufficientLiquidity) {
			err = ErrInsufficientLiquidity
		}
		return nil, nil, fmt.Errorf("PlaceMarketOrder (Exchange): %w", err)
	}
	snap := *order
	return matched, &snap, nil
}

func (e *Exchange) cancelOrder(msg CancelMessage, sig []byte) (*matchengine.Order, error) {
	sb, err := e.symbolBook(msg.Symbol)
	if err != nil {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}
	signer, err := recoverSigner(msg.typedData(e.chainID), sig)
	if err != nil {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}
	if signer != msg.Maker {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrInvalidSignature)
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	resting, ok := sb.book.GetOrder(msg.OrderID)
	if !ok {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrOrderNotFound)
	}
	if resting.Maker != msg.Maker {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrNotOrderOwner)
	}

	order, err := sb.book.CancelOrder(msg.OrderID)
	if err != nil {
		if errors.Is(err, matchengine.ErrOrderNotFound) {
			err = ErrOrderNotFound
		}
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}
	return order, nil
}

// getOrderBook returns a snapshot of sym's book. It is built while the lock
// is held: the book's levels and orders are shared, mutable state, so reading
// them after unlocking would race with incoming orders.
func (e *Exchange) getOrderBook(sym Symbol) (orderBookResponse, error) {
	sb, err := e.symbolBook(sym)
	if err != nil {
		return orderBookResponse{}, fmt.Errorf("GetOrderBook (Exchange): %w", err)
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.book.PurgeExpired()
	return newOrderBookResponse(sb.book, sym), nil
}
