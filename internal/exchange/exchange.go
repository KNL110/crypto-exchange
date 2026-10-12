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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
	"github.com/knl110/crypto-exchange/internal/wal"
)

// Exchange coordinates trading across all configured symbols.
//
// Users are wallet addresses: every order and cancel must be signed by the
// maker (see signing.go). privateKey is the exchange's own operator key, used
// to submit settlement transactions — never a user's key.
type Exchange struct {
	privateKey  *ecdsa.PrivateKey
	chainID     *big.Int
	orderBooks  map[symbol.Symbol]*symbol.Book
	nextOrderID atomic.Uint64

	// now is the exchange's clock. Each operation reads it once and applies
	// entirely at that time (see SymbolBook.now), so it can be replayed exactly.
	now func() time.Time

	// wal durably logs every accepted order and cancel before it is applied,
	// so Recover can rebuild the books after a restart. nil means nothing is
	// persisted (no Recover called).
	wal *wal.WAL

	// usedNonces stops a signed order from being submitted twice. Recover
	// rebuilds it from the WAL.
	noncesMu   sync.Mutex
	usedNonces map[common.Address]map[uint64]struct{}
}

// NewExchange creates an Exchange with an empty order book for each symbol.
// chainID is part of the EIP-712 domain, so signatures from other chains are rejected.
func NewExchange(privateKey string, chainID *big.Int) (*Exchange, error) {
	prvKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key(Exchange): %w", err)
	}
	e := &Exchange{
		orderBooks: make(map[symbol.Symbol]*symbol.Book),
		privateKey: prvKey,
		chainID:    chainID,
		now:        time.Now,
		usedNonces: make(map[common.Address]map[uint64]struct{}),
	}
	for _, sym := range symbol.Configured {
		e.orderBooks[sym] = symbol.NewBook(sym)
	}
	return e, nil
}

func (e *Exchange) symbolBook(sym symbol.Symbol) (*symbol.Book, error) {
	sb, ok := e.orderBooks[sym]
	if !ok {
		return nil, ErrUnknownSymbol
	}
	return sb, nil
}

// Symbols returns the configured trading symbols.
func (e *Exchange) Symbols() []symbol.Symbol {
	symbols := make([]symbol.Symbol, 0, len(e.orderBooks))
	for s := range e.orderBooks {
		symbols = append(symbols, s)
	}
	return symbols
}

// replayed carries what Recover takes from a WAL record instead of
// generating it live: the original time and order ID. nil means live.
type replayed struct {
	ts      int64
	orderID uint64
}

// opTime is the unix time (seconds) an operation runs at.
func (e *Exchange) opTime(rp *replayed) int64 {
	if rp != nil {
		return rp.ts
	}
	return e.now().Unix()
}

// logAction appends rec to the WAL. Replayed actions are already in it.
func (e *Exchange) logAction(rp *replayed, rec wal.Record) error {
	if rp != nil || e.wal == nil {
		return nil
	}
	if _, err := e.wal.Append(rec); err != nil {
		return fmt.Errorf("append wal: %w", err)
	}
	return nil
}

// authorizeOrder checks the maker signed msg, that it has not expired at ts,
// and reserves its nonce. Call releaseNonce if the order is then not accepted.
func (e *Exchange) authorizeOrder(msg signing.Order, sig []byte, ts int64) error {
	signer, err := signing.RecoverSigner(msg.TypedData(e.chainID), sig)
	if err != nil {
		return err
	}
	if signer != msg.Maker {
		return ErrInvalidSignature
	}
	if msg.Expiry <= uint64(ts) {
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

// placeLimitOrder trades the order against the book as far as its limit price
// allows, then rests whatever is left. It returns the fills (possibly none),
// a snapshot of the order, and the maker's own resting orders that self-trade
// prevention cancelled.
func (e *Exchange) placeLimitOrder(msg signing.Order, sig []byte) ([]matchengine.MatchedOrder, *matchengine.Order, []*matchengine.Order, error) {
	return e.placeOrder(msg, sig, false, nil)
}

// placeMarketOrder fills the whole order now, never past its signed worst
// price, or fails with ErrInsufficientLiquidity and changes nothing.
func (e *Exchange) placeMarketOrder(msg signing.Order, sig []byte) ([]matchengine.MatchedOrder, *matchengine.Order, []*matchengine.Order, error) {
	return e.placeOrder(msg, sig, true, nil)
}

// placeOrder is shared by live orders (rp == nil) and WAL replay, so a
// replayed order goes through exactly the same checks and matching.
//
// TODO:consider the performance impact of copying the order struct in all these services
func (e *Exchange) placeOrder(msg signing.Order, sig []byte, market bool, rp *replayed) ([]matchengine.MatchedOrder, *matchengine.Order, []*matchengine.Order, error) {
	op, action := "PlaceLimitOrder (Exchange)", wal.ActionPlaceLimit
	if market {
		op, action = "PlaceMarketOrder (Exchange)", wal.ActionPlaceMarket
	}
	fail := func(err error) ([]matchengine.MatchedOrder, *matchengine.Order, []*matchengine.Order, error) {
		return nil, nil, nil, fmt.Errorf("%s: %w", op, err)
	}

	if msg.IsMarket != market {
		return fail(ErrOrderTypeMismatch)
	}
	if msg.Size == 0 {
		return fail(ErrInvalidSize)
	}
	// For a market order, Price is the worst price the maker accepts;
	// without one it could fill at any price.
	if msg.Price == 0 {
		return fail(ErrInvalidPrice)
	}

	sb, err := e.symbolBook(msg.Symbol)
	if err != nil {
		return fail(err)
	}
	ts := e.opTime(rp)
	if err := e.authorizeOrder(msg, sig, ts); err != nil {
		return fail(err)
	}

	ob := sb.Lock(uint64(ts))
	defer sb.Unlock()

	var id uint64
	if rp != nil {
		id = rp.orderID
	} else {
		id = e.nextOrderID.Add(1)
	}

	// Logged before it touches the book: if the engine then rejects it, replay
	// rejects it the same way, so the rebuilt book still matches.
	if err := e.logAction(rp, wal.Record{
		Timestamp: ts,
		Symbol:    string(msg.Symbol),
		Action:    action,
		OrderID:   id,
		Maker:     msg.Maker.Hex(),
		IsBid:     msg.IsBid,
		Size:      msg.Size,
		Price:     msg.Price,
		Nonce:     msg.Nonce,
		Expiry:    msg.Expiry,
		Signature: hexutil.Encode(sig),
	}); err != nil {
		e.releaseNonce(msg.Maker, msg.Nonce)
		return fail(err)
	}

	order := matchengine.NewOrder(id, msg.IsBid, msg.Size, msg.Maker)
	order.Expiry = msg.Expiry

	var matched []matchengine.MatchedOrder
	var cancelled []*matchengine.Order
	if market {
		matched, cancelled, err = ob.PlaceMarketOrder(order, msg.Price)
	} else {
		matched, cancelled, err = ob.PlaceLimitOrder(order, msg.Price)
	}
	if err != nil {
		e.releaseNonce(msg.Maker, msg.Nonce)
		if errors.Is(err, matchengine.ErrInsufficientLiquidity) {
			err = ErrInsufficientLiquidity
		}
		return fail(err)
	}
	snap := *order
	return matched, &snap, cancelled, nil
}

func (e *Exchange) cancelOrder(msg signing.Cancel, sig []byte) (*matchengine.Order, error) {
	return e.cancel(msg, sig, nil)
}

// cancel is shared by live cancels (rp == nil) and WAL replay.
func (e *Exchange) cancel(msg signing.Cancel, sig []byte, rp *replayed) (*matchengine.Order, error) {
	sb, err := e.symbolBook(msg.Symbol)
	if err != nil {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}
	signer, err := signing.RecoverSigner(msg.TypedData(e.chainID), sig)
	if err != nil {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}
	if signer != msg.Maker {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrInvalidSignature)
	}

	ts := e.opTime(rp)
	ob := sb.Lock(uint64(ts))
	defer sb.Unlock()

	resting, ok := ob.GetOrder(msg.OrderID)
	if !ok {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrOrderNotFound)
	}
	if resting.Maker != msg.Maker {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", ErrNotOrderOwner)
	}

	if err := e.logAction(rp, wal.Record{
		Timestamp: ts,
		Symbol:    string(msg.Symbol),
		Action:    wal.ActionCancel,
		OrderID:   msg.OrderID,
		Maker:     msg.Maker.Hex(),
		Signature: hexutil.Encode(sig),
	}); err != nil {
		return nil, fmt.Errorf("CancelOrder (Exchange): %w", err)
	}

	order, err := ob.CancelOrder(msg.OrderID)
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
func (e *Exchange) getOrderBook(sym symbol.Symbol) (orderBookResponse, error) {
	sb, err := e.symbolBook(sym)
	if err != nil {
		return orderBookResponse{}, fmt.Errorf("GetOrderBook (Exchange): %w", err)
	}
	ob := sb.Lock(uint64(e.now().Unix()))
	defer sb.Unlock()
	ob.PurgeExpired()
	return newOrderBookResponse(ob, sym), nil
}
