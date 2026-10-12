package exchange

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knl110/crypto-exchange/internal/signing/signingtest"
	"github.com/knl110/crypto-exchange/internal/symbol"
	"github.com/knl110/crypto-exchange/internal/wal"
)

// recovered starts a fresh exchange from the WAL at path, as a restart would.
func recovered(t *testing.T, path string, key []byte) *Exchange {
	t.Helper()
	ex := newTestExchange(t)
	if err := ex.Recover(path, key); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	t.Cleanup(func() { ex.Close() })
	return ex
}

func allBooks(t *testing.T, ex *Exchange) map[symbol.Symbol]BookSnapshot {
	t.Helper()
	books := make(map[symbol.Symbol]BookSnapshot)
	for _, sym := range symbol.Configured {
		b, err := ex.OrderBook(sym)
		if err != nil {
			t.Fatalf("getOrderBook(%s): %v", sym, err)
		}
		books[sym] = b
	}
	return books
}

func TestRecoverRebuildsOrderBooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice, bob, carol := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)

	ex := recovered(t, path, nil)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Resting orders on both sides of two symbols.
	_, err := ex.PlaceLimitOrder(alice.Limit(false, 3_500, 10, symETH))
	must(err)
	_, err = ex.PlaceLimitOrder(alice.Limit(false, 3_600, 10, symETH))
	must(err)
	toCancelPlaced, err := ex.PlaceLimitOrder(bob.Limit(true, 3_000, 7, symETH))
	toCancel := toCancelPlaced.Order
	must(err)
	_, err = ex.PlaceLimitOrder(carol.Limit(true, 60_000, 2, symbol.Symbol("BTC")))
	must(err)
	// A crossing limit order (fills 10 @ 3500, 5 rest at 3550) and a market order.
	_, err = ex.PlaceLimitOrder(bob.Limit(true, 3_550, 15, symETH))
	must(err)
	_, err = ex.PlaceMarketOrder(carol.Market(true, 4, 3_600, symETH))
	must(err)
	// A cancel, and a market order that is logged but rejected for liquidity.
	_, err = ex.CancelOrder(bob.Cancel(symETH, toCancel.ID))
	must(err)
	if _, err := ex.PlaceMarketOrder(carol.Market(true, 1_000, 10_000, symETH)); !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("placeMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}

	before := allBooks(t, ex)
	lastID := ex.nextOrderID.Load()
	must(ex.Close())

	// Restart.
	ex2 := recovered(t, path, nil)
	if after := allBooks(t, ex2); !reflect.DeepEqual(before, after) {
		t.Fatalf("books after recovery differ:\nbefore %+v\nafter  %+v", before, after)
	}
	if got := ex2.nextOrderID.Load(); got != lastID {
		t.Fatalf("nextOrderID = %d, want %d", got, lastID)
	}

	// New orders continue after the recovered IDs and are logged too.
	oPlaced, err := ex2.PlaceLimitOrder(alice.Limit(false, 3_700, 1, symETH))
	o := oPlaced.Order
	must(err)
	if o.ID <= lastID {
		t.Fatalf("new order ID %d, want > %d", o.ID, lastID)
	}
	afterNew := allBooks(t, ex2)
	must(ex2.Close())
	if again := allBooks(t, recovered(t, path, nil)); !reflect.DeepEqual(afterNew, again) {
		t.Fatalf("second recovery differs:\nwant %+v\ngot  %+v", afterNew, again)
	}
}

func TestRecoverRebuildsUsedNonces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)

	ex := recovered(t, path, nil)
	msg, sig := alice.Limit(true, 3_000, 10, symETH)
	if _, err := ex.PlaceLimitOrder(msg, sig); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	ex.Close()

	// The same signed order replayed after a restart is still rejected.
	ex2 := recovered(t, path, nil)
	if _, err := ex2.PlaceLimitOrder(msg, sig); !errors.Is(err, ErrNonceUsed) {
		t.Fatalf("replayed order after restart: err = %v, want ErrNonceUsed", err)
	}
}

func TestRecoverReplaysAtOriginalTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)
	start := time.Now()

	ex := recovered(t, path, nil)
	ex.now = func() time.Time { return start }
	msg := alice.NewOrder(true, 10, 3_000, symETH)
	msg.Expiry = uint64(start.Add(10 * time.Second).Unix())
	if _, err := ex.PlaceLimitOrder(msg, alice.Sign(msg.TypedData(testChainID))); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	ex.Close()

	// Restart after the order expired: replay must still accept it (it was
	// valid when placed), and only then does it expire out of the book.
	ex2 := newTestExchange(t)
	ex2.now = func() time.Time { return start.Add(time.Hour) }
	if err := ex2.Recover(path, nil); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	defer ex2.Close()
	if book, _ := ex2.OrderBook(symETH); len(book.Bids) != 0 {
		t.Fatalf("expired order still shown after recovery: %+v", book.Bids)
	}
}

func TestRecoverRejectsTamperedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)

	ex := recovered(t, path, nil)
	if _, err := ex.PlaceLimitOrder(alice.Limit(true, 3_000, 10, symETH)); err != nil {
		t.Fatalf("placeLimitOrder: %v", err)
	}
	ex.Close()

	// Even with no HMAC key, changing the size breaks the maker's signature.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"size":10`, `"size":1000`, 1)
	if tampered == string(data) {
		t.Fatalf("test setup: size field not found in %s", data)
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := newTestExchange(t).Recover(path, nil); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("Recover tampered wal: err = %v, want ErrInvalidSignature", err)
	}
}

func TestRecoverWithHMACDetectsDeletedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-hmac-key")
	alice := signingtest.NewTrader(t, testChainID)

	ex := recovered(t, path, key)
	for _, p := range []uint64{3_000, 3_100, 3_200} {
		if _, err := ex.PlaceLimitOrder(alice.Limit(true, p, 10, symETH)); err != nil {
			t.Fatalf("placeLimitOrder: %v", err)
		}
	}
	ex.Close()

	// Deleting a validly signed record can't be caught by order signatures, only by the chain.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	if err := os.WriteFile(path, []byte(lines[0]+lines[2]), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := newTestExchange(t).Recover(path, key); !errors.Is(err, wal.ErrIntegrityViolation) {
		t.Fatalf("Recover with deleted record: err = %v, want wal.ErrIntegrityViolation", err)
	}
}
