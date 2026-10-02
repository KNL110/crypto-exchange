package exchange

import (
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	symETH = Symbol("ETH")
	// symUnknown is not configured on the exchange.
	symUnknown = Symbol("DOGE")
)

// newTestExchange returns an Exchange backed by a freshly generated private key.
func newTestExchange(t *testing.T) *Exchange {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	ex, err := NewExchange(hex.EncodeToString(crypto.FromECDSA(key)))
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}
	return ex
}

// bookVolume returns the total resting size on each side of sym's book.
func bookVolume(t *testing.T, ex *Exchange, sym Symbol) (bids, asks int64) {
	t.Helper()
	sb, err := ex.symbolBook(sym)
	if err != nil {
		t.Fatalf("symbolBook: %v", err)
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.book.BidTotalVolume(), sb.book.AskTotalVolume()
}

func TestNewExchangeRegistersConfiguredSymbols(t *testing.T) {
	ex := newTestExchange(t)

	if syms := ex.Symbols(); len(syms) != len(symbols) {
		t.Fatalf("Symbols() = %v, want %v", syms, symbols)
	}
	for _, sym := range symbols {
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

	order, err := ex.placeLimitOrder(false, 3_500, 10, symETH)
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if order.ID == 0 {
		t.Fatalf("order ID not assigned")
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

	a, err := ex.placeLimitOrder(true, 100, 1, symETH)
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	b, err := ex.placeLimitOrder(true, 100, 1, symETH)
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("duplicate order ID %d", a.ID)
	}
}

func TestPlaceLimitOrderRejectsBadInput(t *testing.T) {
	ex := newTestExchange(t)

	if _, err := ex.placeLimitOrder(true, 100, 0, symETH); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("size=0: err = %v, want ErrInvalidSize", err)
	}
	if _, err := ex.placeLimitOrder(true, 0, 10, symETH); !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("price=0: err = %v, want ErrInvalidPrice", err)
	}
	if _, err := ex.placeLimitOrder(true, 100, 10, symUnknown); !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
}

func TestPlaceMarketOrderFillsAcrossLevels(t *testing.T) {
	ex := newTestExchange(t)

	for _, p := range []float64{3_500, 3_450, 3_400} {
		if _, err := ex.placeLimitOrder(false, p, 10, symETH); err != nil {
			t.Fatalf("placeLimitOrder: %v", err)
		}
	}

	matches, order, err := ex.placeMarketOrder(true, 15, symETH)
	if err != nil {
		t.Fatalf("placeMarketOrder: %v", err)
	}
	if !order.IsFilled() {
		t.Fatalf("market order not fully filled, remaining size %d", order.Size)
	}

	var filled int64
	for _, m := range matches {
		filled += m.SizeFilled
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

func TestPlaceMarketOrderRejectsBadInput(t *testing.T) {
	ex := newTestExchange(t)

	if _, _, err := ex.placeMarketOrder(true, 0, symETH); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("size=0: err = %v, want ErrInvalidSize", err)
	}
	if _, _, err := ex.placeMarketOrder(true, 10, symUnknown); !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
}

func TestPlaceMarketOrderInsufficientLiquidityLeavesBookUntouched(t *testing.T) {
	ex := newTestExchange(t)

	if _, err := ex.placeLimitOrder(false, 3_500, 5, symETH); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	_, _, err := ex.placeMarketOrder(true, 100, symETH)
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("placeMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}

	if _, asks := bookVolume(t, ex, symETH); asks != 5 {
		t.Fatalf("ask volume = %d, want resting order untouched at 5", asks)
	}
}

func TestCancelOrderRemovesFromBook(t *testing.T) {
	ex := newTestExchange(t)

	order, err := ex.placeLimitOrder(true, 3_000, 10, symETH)
	if err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}

	cancelled, err := ex.cancelOrder(symETH, order.ID)
	if err != nil {
		t.Fatalf("cancelOrder: %v", err)
	}
	if cancelled.ID != order.ID {
		t.Fatalf("cancelled ID = %d, want %d", cancelled.ID, order.ID)
	}

	if bids, _ := bookVolume(t, ex, symETH); bids != 0 {
		t.Fatalf("bid volume after cancel = %d, want 0", bids)
	}

	if _, err := ex.cancelOrder(symETH, order.ID); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("double cancel: err = %v, want ErrOrderNotFound", err)
	}
	if _, err := ex.cancelOrder(symUnknown, order.ID); !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("unknown symbol: err = %v, want ErrUnknownSymbol", err)
	}
}

func TestConcurrentOrdersSameSymbol(t *testing.T) {
	ex := newTestExchange(t)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			// Bids below 3000, asks above it, so nothing crosses.
			isBid := i%2 == 0
			price := float64(3_001 + i)
			if isBid {
				price = float64(2_999 - i)
			}
			if _, err := ex.placeLimitOrder(isBid, price, 1, symETH); err != nil {
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
