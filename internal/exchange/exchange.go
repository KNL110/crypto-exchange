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
	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
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
	// entirely at that time (see symbol.Book.Lock), so it can be replayed exactly.
	now func() time.Time

	// journal durably records every accepted order and cancel before it is
	// applied, so the books can be rebuilt with Replay. nil records nothing.
	journal Journal

	// usedNonces stops a signed order from being submitted twice. Replay
	// rebuilds it.
	noncesMu   sync.Mutex
	usedNonces map[common.Address]map[uint64]struct{}
}

// Option configures an Exchange.
type Option func(*Exchange)

// WithClock replaces the exchange's clock (time.Now by default).
func WithClock(now func() time.Time) Option {
	return func(e *Exchange) { e.now = now }
}

// NewExchange creates an Exchange with an empty order book for each symbol.
// chainID is part of the EIP-712 domain, so signatures from other chains are rejected.
func NewExchange(privateKey string, chainID *big.Int, opts ...Option) (*Exchange, error) {
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
	for _, opt := range opts {
		opt(e)
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

// Symbols returns the traded symbols, in configured order.
func (e *Exchange) Symbols() []symbol.Symbol {
	symbols := make([]symbol.Symbol, 0, len(e.orderBooks))
	for _, s := range symbol.Configured {
		if _, ok := e.orderBooks[s]; ok {
			symbols = append(symbols, s)
		}
	}
	return symbols
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

// Placed is the outcome of placing an order.
type Placed struct {
	// Order is a snapshot of the order after matching: Size is what is
	// still unfilled (and resting, for a limit order), Filled what traded.
	Order *matchengine.Order
	// Matches is every fill, best price first.
	Matches []matchengine.MatchedOrder
	// Cancelled lists the maker's own resting orders that self-trade
	// prevention removed instead of trading against.
	Cancelled []*matchengine.Order
}

// PlaceLimitOrder trades a signed limit order against the book as far as its
// limit price allows, then rests whatever is left.
func (e *Exchange) PlaceLimitOrder(msg signing.Order, sig []byte) (Placed, error) {
	return e.placeOrder(msg, sig, false, nil)
}

// PlaceMarketOrder fills the whole signed market order now, never past its
// worst price, or fails with ErrInsufficientLiquidity and changes nothing.
func (e *Exchange) PlaceMarketOrder(msg signing.Order, sig []byte) (Placed, error) {
	return e.placeOrder(msg, sig, true, nil)
}

// placeOrder is shared by live orders (rp == nil) and WAL replay, so a
// replayed order goes through exactly the same checks and matching.
//
// TODO:consider the performance impact of copying the order struct in all these services
func (e *Exchange) placeOrder(msg signing.Order, sig []byte, market bool, rp *replayed) (Placed, error) {
	op, kind := "PlaceLimitOrder (Exchange)", ActionPlaceLimit
	if market {
		op, kind = "PlaceMarketOrder (Exchange)", ActionPlaceMarket
	}
	fail := func(err error) (Placed, error) {
		return Placed{}, fmt.Errorf("%s: %w", op, err)
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

	// Journaled before it touches the book: if the engine then rejects it,
	// replay rejects it the same way, so the rebuilt book still matches.
	if err := e.logAction(rp, Action{Kind: kind, Time: ts, OrderID: id, Order: msg, Signature: sig}); err != nil {
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
	return Placed{Order: &snap, Matches: matched, Cancelled: cancelled}, nil
}

// CancelOrder cancels one of the maker's resting orders. The cancel must be
// signed by the order's maker.
func (e *Exchange) CancelOrder(msg signing.Cancel, sig []byte) (*matchengine.Order, error) {
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

	if err := e.logAction(rp, Action{Kind: ActionCancel, Time: ts, OrderID: msg.OrderID, Cancel: msg, Signature: sig}); err != nil {
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

// BookSnapshot is a copy of one symbol's order book, taken under its lock,
// so it is safe to read after the lock is released.
type BookSnapshot struct {
	Symbol symbol.Symbol
	Bids   []LevelSnapshot // best (highest) price first
	Asks   []LevelSnapshot // best (lowest) price first
}

// LevelSnapshot is one price level of a BookSnapshot.
type LevelSnapshot struct {
	Price       uint64
	TotalVolume uint64
	Orders      []OrderSnapshot // oldest first, the order they fill in
}

// OrderSnapshot is a resting order as of a BookSnapshot.
type OrderSnapshot struct {
	ID     uint64
	Maker  common.Address
	IsBid  bool
	Size   uint64 // unfilled
	Filled uint64
	Expiry uint64
}

// OrderBook returns a snapshot of sym's book without expired orders. It is
// built while the lock is held: the book's levels and orders are shared,
// mutable state, so reading them after unlocking would race with incoming orders.
func (e *Exchange) OrderBook(sym symbol.Symbol) (BookSnapshot, error) {
	sb, err := e.symbolBook(sym)
	if err != nil {
		return BookSnapshot{}, fmt.Errorf("GetOrderBook (Exchange): %w", err)
	}
	ob := sb.Lock(uint64(e.now().Unix()))
	defer sb.Unlock()
	ob.PurgeExpired()
	return BookSnapshot{Symbol: sym, Bids: snapshotLevels(ob.Bids()), Asks: snapshotLevels(ob.Asks())}, nil
}

func snapshotLevels(limits []*matchengine.Limit) []LevelSnapshot {
	levels := make([]LevelSnapshot, 0, len(limits))
	for _, lim := range limits {
		orders := lim.Orders()
		level := LevelSnapshot{Price: lim.Price, TotalVolume: lim.TotalVolume, Orders: make([]OrderSnapshot, 0, len(orders))}
		for _, o := range orders {
			level.Orders = append(level.Orders, OrderSnapshot{
				ID: o.ID, Maker: o.Maker, IsBid: o.IsBid, Size: o.Size, Filled: o.Filled, Expiry: o.Expiry,
			})
		}
		levels = append(levels, level)
	}
	return levels
}
