package exchange

import (
	"encoding/hex"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/signing/signingtest"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

const (
	symETH = symbol.Symbol("ETH")
	// symUnknown is not configured on the exchange.
	symUnknown = symbol.Symbol("DOGE")
)

var testChainID = big.NewInt(1337)

// newTestExchange returns an Exchange backed by a freshly generated operator key.
func newTestExchange(t *testing.T) *Exchange {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	ex, err := NewExchange(hex.EncodeToString(crypto.FromECDSA(key)), testChainID)
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}
	return ex
}

// bookVolume returns the total resting size on each side of sym's book.
func bookVolume(t *testing.T, ex *Exchange, sym symbol.Symbol) (bids, asks uint64) {
	t.Helper()
	sb, err := ex.symbolBook(sym)
	if err != nil {
		t.Fatalf("symbolBook: %v", err)
	}
	ob := sb.Lock(uint64(time.Now().Unix()))
	defer sb.Unlock()
	return ob.BidTotalVolume(), ob.AskTotalVolume()
}

func TestNewExchangeRegistersConfiguredSymbols(t *testing.T) {
	ex := newTestExchange(t)

	if syms := ex.Symbols(); len(syms) != len(symbol.Configured) {
		t.Fatalf("Symbols() = %v, want %v", syms, symbol.Configured)
	}
	for _, sym := range symbol.Configured {
		if _, err := ex.symbolBook(sym); err != nil {
			t.Fatalf("symbolBook(%s): %v", sym, err)
		}
	}
	if _, err := ex.symbolBook(symUnknown); !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("symbolBook(%s): err = %v, want ErrUnknownSymbol", symUnknown, err)
	}
}

func TestPlaceLimitOrderRests(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	_, order, _, err := ex.placeLimitOrder(alice.Limit(false, 3_500, 10, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if order.ID == 0 {
		t.Fatalf("order ID not assigned")
	}
	if order.Maker != alice.Addr {
		t.Fatalf("order maker = %s, want %s", order.Maker, alice.Addr)
	}
	if order.Limit == nil || order.Limit.Price != 3_500 {
		t.Fatalf("order not resting at 3500: %+v", order.Limit)
	}

	bids, asks := bookVolume(t, ex, symETH)
	if bids != 0 || asks != 10 {
		t.Fatalf("volume = bids %d / asks %d, want 0 / 10", bids, asks)
	}
}

func TestPlaceLimitOrderAssignsUniqueIDs(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	_, a, _, err := ex.placeLimitOrder(alice.Limit(true, 100, 1, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	_, b, _, err := ex.placeLimitOrder(alice.Limit(true, 100, 1, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("duplicate order ID %d", a.ID)
	}
}

func TestPlaceLimitOrderRejectsBadInput(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	if _, _, _, err := ex.placeLimitOrder(alice.Limit(true, 100, 0, symETH)); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("size=0: err = %v, want ErrInvalidSize", err)
	}
	if _, _, _, err := ex.placeLimitOrder(alice.Limit(true, 0, 10, symETH)); !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("price=0: err = %v, want ErrInvalidPrice", err)
	}
	if _, _, _, err := ex.placeLimitOrder(alice.Limit(true, 100, 10, symUnknown)); !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
}

func TestPlaceOrderRejectsBadSignature(t *testing.T) {
	ex := newTestExchange(t)
	alice, mallory := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	// Mallory signs an order claiming to be Alice.
	msg := alice.NewOrder(true, 10, 100, symETH)
	if _, _, _, err := ex.placeLimitOrder(msg, mallory.Sign(msg.TypedData(testChainID))); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("wrong signer: err = %v, want ErrInvalidSignature", err)
	}

	// Alice's signature no longer matches once the size is tampered with.
	msg, sig := alice.Limit(true, 100, 10, symETH)
	msg.Size = 1_000
	if _, _, _, err := ex.placeLimitOrder(msg, sig); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("tampered order: err = %v, want ErrInvalidSignature", err)
	}

	// A signature for another chain must not be accepted here.
	msg = alice.NewOrder(true, 10, 100, symETH)
	if _, _, _, err := ex.placeLimitOrder(msg, alice.Sign(msg.TypedData(big.NewInt(1)))); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("other chain: err = %v, want ErrInvalidSignature", err)
	}

	if _, _, _, err := ex.placeLimitOrder(msg, []byte{1, 2, 3}); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("malformed signature: err = %v, want ErrInvalidSignature", err)
	}

	if bids, _ := bookVolume(t, ex, symETH); bids != 0 {
		t.Fatalf("bid volume = %d, want 0 after rejected orders", bids)
	}
}

func TestPlaceOrderRejectsExpired(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	msg := alice.NewOrder(true, 10, 100, symETH)
	msg.Expiry = uint64(time.Now().Add(-time.Minute).Unix())
	if _, _, _, err := ex.placeLimitOrder(msg, alice.Sign(msg.TypedData(testChainID))); !errors.Is(err, ErrOrderExpired) {
		t.Fatalf("expired order: err = %v, want ErrOrderExpired", err)
	}
}

func TestPlaceOrderRejectsReplay(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	msg, sig := alice.Limit(true, 100, 10, symETH)
	if _, _, _, err := ex.placeLimitOrder(msg, sig); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if _, _, _, err := ex.placeLimitOrder(msg, sig); !errors.Is(err, ErrNonceUsed) {
		t.Fatalf("replayed order: err = %v, want ErrNonceUsed", err)
	}
	if bids, _ := bookVolume(t, ex, symETH); bids != 10 {
		t.Fatalf("bid volume = %d, want 10 (replay must not rest)", bids)
	}
}

func TestPlaceMarketOrderFillsAcrossLevels(t *testing.T) {
	ex := newTestExchange(t)
	alice, bob := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	for _, p := range []uint64{3_500, 3_450, 3_400} {
		if _, _, _, err := ex.placeLimitOrder(alice.Limit(false, p, 10, symETH)); err != nil {
			t.Fatalf("placeLimitOrder: %v", err)
		}
	}

	matches, order, _, err := ex.placeMarketOrder(bob.Market(true, 15, 10_000, symETH))
	if err != nil {
		t.Fatalf("placeMarketOrder: %v", err)
	}
	if !order.IsFilled() {
		t.Fatalf("market order not fully filled, remaining size %d", order.Size)
	}

	var filled uint64
	for _, m := range matches {
		filled += m.SizeFilled
		if m.Bid.Maker != bob.Addr || m.Ask.Maker != alice.Addr {
			t.Fatalf("match makers = bid %s / ask %s, want bob / alice", m.Bid.Maker, m.Ask.Maker)
		}
	}
	if filled != 15 {
		t.Fatalf("total filled = %d, want 15", filled)
	}
	// Best ask (lowest price) must be consumed first.
	if len(matches) == 0 || matches[0].Price != 3_400 {
		t.Fatalf("first fill = %+v, want price 3400", matches)
	}

	if _, asks := bookVolume(t, ex, symETH); asks != 15 {
		t.Fatalf("remaining ask volume = %d, want 15", asks)
	}
}

func TestPlaceLimitOrderCrossesBook(t *testing.T) {
	ex := newTestExchange(t)
	alice, bob := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	if _, _, _, err := ex.placeLimitOrder(alice.Limit(false, 3_500, 10, symETH)); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	// Bob's buy at 3600 crosses Alice's 3500 ask: 10 fill at 3500, 5 rest at 3600.
	matches, order, _, err := ex.placeLimitOrder(bob.Limit(true, 3_600, 15, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if len(matches) != 1 || matches[0].Price != 3_500 || matches[0].SizeFilled != 10 {
		t.Fatalf("matches = %+v, want one fill of 10 at 3500", matches)
	}
	if matches[0].Bid.Maker != bob.Addr || matches[0].Ask.Maker != alice.Addr {
		t.Fatalf("match makers = bid %s / ask %s, want bob / alice", matches[0].Bid.Maker, matches[0].Ask.Maker)
	}
	if order.Filled != 10 || order.Size != 5 || order.Limit == nil || order.Limit.Price != 3_600 {
		t.Fatalf("order = filled %d, size %d, limit %+v; want 10 filled, 5 resting at 3600", order.Filled, order.Size, order.Limit)
	}

	bids, asks := bookVolume(t, ex, symETH)
	if bids != 5 || asks != 0 {
		t.Fatalf("volume = bids %d / asks %d, want 5 / 0", bids, asks)
	}
}

func TestPlaceMarketOrderRejectsBadInput(t *testing.T) {
	ex := newTestExchange(t)
	bob := signingtest.NewTrader(t, testChainID)

	if _, _, _, err := ex.placeMarketOrder(bob.Market(true, 0, 10_000, symETH)); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("size=0: err = %v, want ErrInvalidSize", err)
	}
	if _, _, _, err := ex.placeMarketOrder(bob.Market(true, 10, 10_000, symUnknown)); !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
	// A market order must carry a worst price, or it could fill at any price.
	if _, _, _, err := ex.placeMarketOrder(bob.Market(true, 10, 0, symETH)); !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("worst price=0: err = %v, want ErrInvalidPrice", err)
	}
	// A signed limit order must not be executable as a market order, and vice versa.
	if _, _, _, err := ex.placeMarketOrder(bob.Limit(true, 100, 10, symETH)); !errors.Is(err, ErrOrderTypeMismatch) {
		t.Errorf("limit order on market endpoint: err = %v, want ErrOrderTypeMismatch", err)
	}
	if _, _, _, err := ex.placeLimitOrder(bob.Market(true, 10, 100, symETH)); !errors.Is(err, ErrOrderTypeMismatch) {
		t.Errorf("market order on limit endpoint: err = %v, want ErrOrderTypeMismatch", err)
	}
}

func TestPlaceMarketOrderInsufficientLiquidityLeavesBookUntouched(t *testing.T) {
	ex := newTestExchange(t)
	alice, bob := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	if _, _, _, err := ex.placeLimitOrder(alice.Limit(false, 3_500, 5, symETH)); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	msg, sig := bob.Market(true, 100, 10_000, symETH)
	if _, _, _, err := ex.placeMarketOrder(msg, sig); !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("placeMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}

	if _, asks := bookVolume(t, ex, symETH); asks != 5 {
		t.Fatalf("ask volume = %d, want resting order untouched at 5", asks)
	}

	// The rejected order's nonce is released, so the same signed order can be retried.
	if _, _, _, err := ex.placeLimitOrder(alice.Limit(false, 3_500, 95, symETH)); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if _, _, _, err := ex.placeMarketOrder(msg, sig); err != nil {
		t.Fatalf("retry placeMarketOrder: %v", err)
	}
}

func TestCancelOrderRemovesFromBook(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	_, order, _, err := ex.placeLimitOrder(alice.Limit(true, 3_000, 10, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	cancelled, err := ex.cancelOrder(alice.Cancel(symETH, order.ID))
	if err != nil {
		t.Fatalf("cancelOrder: %v", err)
	}
	if cancelled.ID != order.ID {
		t.Fatalf("cancelled ID = %d, want %d", cancelled.ID, order.ID)
	}

	if bids, _ := bookVolume(t, ex, symETH); bids != 0 {
		t.Fatalf("bid volume after cancel = %d, want 0", bids)
	}

	if _, err := ex.cancelOrder(alice.Cancel(symETH, order.ID)); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("double cancel: err = %v, want ErrOrderNotFound", err)
	}
	if _, err := ex.cancelOrder(alice.Cancel(symUnknown, order.ID)); !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
}

func TestCancelOrderRequiresOwner(t *testing.T) {
	ex := newTestExchange(t)
	alice, mallory := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	_, order, _, err := ex.placeLimitOrder(alice.Limit(true, 3_000, 10, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	// Mallory validly signs a cancel for Alice's order.
	if _, err := ex.cancelOrder(mallory.Cancel(symETH, order.ID)); !errors.Is(err, ErrNotOrderOwner) {
		t.Fatalf("cancel by non-owner: err = %v, want ErrNotOrderOwner", err)
	}

	// Mallory claims to be Alice but can't produce her signature.
	msg := signing.Cancel{Maker: alice.Addr, Symbol: symETH, OrderID: order.ID}
	if _, err := ex.cancelOrder(msg, mallory.Sign(msg.TypedData(testChainID))); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("forged cancel: err = %v, want ErrInvalidSignature", err)
	}

	if bids, _ := bookVolume(t, ex, symETH); bids != 10 {
		t.Fatalf("bid volume = %d, want order still resting", bids)
	}
}

func TestConcurrentOrdersSameSymbol(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			// Bids below 3000, asks above it, so nothing crosses.
			isBid := i%2 == 0
			price := uint64(3_001 + i)
			if isBid {
				price = uint64(2_999 - i)
			}
			if _, _, _, err := ex.placeLimitOrder(alice.Limit(isBid, price, 1, symETH)); err != nil {
				t.Errorf("placeLimitOrder: %v", err)
			}
		}(i)
	}
	wg.Wait()

	bids, asks := bookVolume(t, ex, symETH)
	if bids+asks != n {
		t.Fatalf("total resting size = %d, want %d", bids+asks, n)
	}
}

// Run with -race: reading the book must not race with orders being placed.
func TestOrderBookReadDuringWrites(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			ex.placeLimitOrder(alice.Limit(i%2 == 0, uint64(1_000+i%7*10), 1, symETH))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := ex.getOrderBook(symETH); err != nil {
				t.Errorf("getOrderBook: %v", err)
			}
		}
	}()
	wg.Wait()
}

func TestSelfTradeIsPrevented(t *testing.T) {
	ex := newTestExchange(t)
	alice := signingtest.NewTrader(t, testChainID)

	ask, err := func() (uint64, error) {
		_, o, _, err := ex.placeLimitOrder(alice.Limit(false, 3_500, 10, symETH))
		if err != nil {
			return 0, err
		}
		return o.ID, nil
	}()
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	// Alice's buy crosses her own ask: no trade, her ask is cancelled (and reported), the buy rests.
	matches, _, cancelled, err := ex.placeLimitOrder(alice.Limit(true, 3_600, 10, symETH))
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, want none (self-trade)", matches)
	}
	if len(cancelled) != 1 || cancelled[0].ID != ask {
		t.Fatalf("cancelled = %+v, want alice's ask %d", cancelled, ask)
	}
	if _, err := ex.cancelOrder(alice.Cancel(symETH, ask)); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("own ask should be gone: cancel err = %v, want ErrOrderNotFound", err)
	}
	if bids, asks := bookVolume(t, ex, symETH); bids != 10 || asks != 0 {
		t.Fatalf("volume = bids %d / asks %d, want 10 / 0", bids, asks)
	}
}

func TestOrderBookHidesOrdersOnceExpired(t *testing.T) {
	ex := newTestExchange(t)
	alice, bob := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)
	start := time.Now()
	ex.now = func() time.Time { return start }

	// Both sides of the book get an order that expires in 10s.
	for _, tr := range []struct {
		trader *signingtest.Trader
		isBid  bool
		price  uint64
	}{{alice, true, 3_000}, {bob, false, 4_000}} {
		msg := tr.trader.NewOrder(tr.isBid, 10, tr.price, symETH)
		msg.Expiry = uint64(start.Add(10 * time.Second).Unix())
		if _, _, _, err := ex.placeLimitOrder(msg, tr.trader.Sign(msg.TypedData(testChainID))); err != nil {
			t.Fatalf("placeLimitOrder: %v", err)
		}
	}

	book, err := ex.getOrderBook(symETH)
	if err != nil {
		t.Fatalf("getOrderBook: %v", err)
	}
	if len(book.Bids) != 1 || len(book.Asks) != 1 {
		t.Fatalf("before expiry: %d bids / %d asks, want 1 / 1", len(book.Bids), len(book.Asks))
	}

	ex.now = func() time.Time { return start.Add(time.Minute) }
	book, err = ex.getOrderBook(symETH)
	if err != nil {
		t.Fatalf("getOrderBook: %v", err)
	}
	if len(book.Bids) != 0 || len(book.Asks) != 0 {
		t.Fatalf("after expiry: %d bids / %d asks, want 0 / 0", len(book.Bids), len(book.Asks))
	}
}
