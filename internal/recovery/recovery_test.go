package recovery

import (
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/signing/signingtest"
	"github.com/knl110/crypto-exchange/internal/symbol"
	"github.com/knl110/crypto-exchange/internal/wal"
)

const symETH = symbol.Symbol("ETH")

var testChainID = big.NewInt(1337)

func newExchange(t *testing.T, opts ...exchange.Option) *exchange.Exchange {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	ex, err := exchange.NewExchange(hex.EncodeToString(crypto.FromECDSA(key)), testChainID, opts...)
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}
	return ex
}

// start boots a fresh exchange from the WAL at path, as a (re)start would.
// stop closes the WAL, as a shutdown would.
func start(t *testing.T, path string, key []byte, opts ...exchange.Option) (ex *exchange.Exchange, stop func()) {
	t.Helper()
	ex = newExchange(t, opts...)
	w, err := Recover(ex, path, key)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	closed := false
	stop = func() {
		if !closed {
			closed = true
			if err := w.Close(); err != nil {
				t.Errorf("close wal: %v", err)
			}
		}
	}
	t.Cleanup(stop)
	return ex, stop
}

func allBooks(t *testing.T, ex *exchange.Exchange) map[symbol.Symbol]exchange.BookSnapshot {
	t.Helper()
	books := make(map[symbol.Symbol]exchange.BookSnapshot)
	for _, sym := range symbol.Configured {
		b, err := ex.OrderBook(sym)
		if err != nil {
			t.Fatalf("OrderBook(%s): %v", sym, err)
		}
		books[sym] = b
	}
	return books
}

// maxLoggedOrderID is the highest order ID in the WAL, including orders
// that were logged and then rejected.
func maxLoggedOrderID(t *testing.T, path string) uint64 {
	t.Helper()
	var max uint64
	if _, _, _, err := wal.Replay(path, nil, func(r wal.Record) error {
		if r.OrderID > max {
			max = r.OrderID
		}
		return nil
	}); err != nil {
		t.Fatalf("read wal: %v", err)
	}
	return max
}

func TestRecoverRebuildsOrderBooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice, bob, carol := signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID), signingtest.NewTrader(t, testChainID)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	ex, stop := start(t, path, nil)
	// Resting orders on both sides of two symbols.
	_, err := ex.PlaceLimitOrder(alice.Limit(false, 3_500, 10, symETH))
	must(err)
	_, err = ex.PlaceLimitOrder(alice.Limit(false, 3_600, 10, symETH))
	must(err)
	toCancel, err := ex.PlaceLimitOrder(bob.Limit(true, 3_000, 7, symETH))
	must(err)
	_, err = ex.PlaceLimitOrder(carol.Limit(true, 60_000, 2, symbol.Symbol("BTC")))
	must(err)
	// A crossing limit order (fills 10 @ 3500, 5 rest at 3550) and a market order.
	_, err = ex.PlaceLimitOrder(bob.Limit(true, 3_550, 15, symETH))
	must(err)
	_, err = ex.PlaceMarketOrder(carol.Market(true, 4, 3_600, symETH))
	must(err)
	// A cancel, and a market order that is logged but rejected for liquidity.
	_, err = ex.CancelOrder(bob.Cancel(symETH, toCancel.Order.ID))
	must(err)
	if _, err := ex.PlaceMarketOrder(carol.Market(true, 1_000, 10_000, symETH)); !errors.Is(err, exchange.ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}

	before := allBooks(t, ex)
	stop()

	// Restart.
	ex2, stop2 := start(t, path, nil)
	if after := allBooks(t, ex2); !reflect.DeepEqual(before, after) {
		t.Fatalf("books after recovery differ:\nbefore %+v\nafter  %+v", before, after)
	}

	// New orders continue after every recovered ID (even the rejected order's) and are logged too.
	lastID := maxLoggedOrderID(t, path)
	placed, err := ex2.PlaceLimitOrder(alice.Limit(false, 3_700, 1, symETH))
	must(err)
	if placed.Order.ID <= lastID {
		t.Fatalf("new order ID %d, want > %d", placed.Order.ID, lastID)
	}
	afterNew := allBooks(t, ex2)
	stop2()

	ex3, _ := start(t, path, nil)
	if again := allBooks(t, ex3); !reflect.DeepEqual(afterNew, again) {
		t.Fatalf("second recovery differs:\nwant %+v\ngot  %+v", afterNew, again)
	}
}

func TestRecoverRebuildsUsedNonces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)

	ex, stop := start(t, path, nil)
	msg, sig := alice.Limit(true, 3_000, 10, symETH)
	if _, err := ex.PlaceLimitOrder(msg, sig); err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	stop()

	// The same signed order replayed after a restart is still rejected.
	ex2, _ := start(t, path, nil)
	if _, err := ex2.PlaceLimitOrder(msg, sig); !errors.Is(err, exchange.ErrNonceUsed) {
		t.Fatalf("replayed order after restart: err = %v, want ErrNonceUsed", err)
	}
}

func TestRecoverReplaysAtOriginalTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)
	t0 := time.Now()

	ex, stop := start(t, path, nil, exchange.WithClock(func() time.Time { return t0 }))
	msg := alice.NewOrder(true, 10, 3_000, symETH)
	msg.Expiry = uint64(t0.Add(10 * time.Second).Unix())
	if _, err := ex.PlaceLimitOrder(msg, alice.Sign(msg.TypedData(testChainID))); err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	stop()

	// Restart after the order expired: replay must still accept it (it was
	// valid when placed), and only then does it expire out of the book.
	ex2, _ := start(t, path, nil, exchange.WithClock(func() time.Time { return t0.Add(time.Hour) }))
	if book, _ := ex2.OrderBook(symETH); len(book.Bids) != 0 {
		t.Fatalf("expired order still shown after recovery: %+v", book.Bids)
	}
}

func TestRecoverRejectsTamperedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	alice := signingtest.NewTrader(t, testChainID)

	ex, stop := start(t, path, nil)
	if _, err := ex.PlaceLimitOrder(alice.Limit(true, 3_000, 10, symETH)); err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	stop()

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

	if _, err := Recover(newExchange(t), path, nil); !errors.Is(err, exchange.ErrInvalidSignature) {
		t.Fatalf("Recover tampered wal: err = %v, want ErrInvalidSignature", err)
	}
}

func TestRecoverWithHMACDetectsDeletedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-hmac-key")
	alice := signingtest.NewTrader(t, testChainID)

	ex, stop := start(t, path, key)
	for _, p := range []uint64{3_000, 3_100, 3_200} {
		if _, err := ex.PlaceLimitOrder(alice.Limit(true, p, 10, symETH)); err != nil {
			t.Fatalf("PlaceLimitOrder: %v", err)
		}
	}
	stop()

	// Deleting a validly signed record can't be caught by order signatures, only by the chain.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	if err := os.WriteFile(path, []byte(lines[0]+lines[2]), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Recover(newExchange(t), path, key); !errors.Is(err, wal.ErrIntegrityViolation) {
		t.Fatalf("Recover with deleted record: err = %v, want wal.ErrIntegrityViolation", err)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	alice := signingtest.NewTrader(t, testChainID)
	order, sig := alice.Market(false, 5, 3_000, symETH)
	cancel, csig := alice.Cancel(symETH, 9)

	for _, a := range []exchange.Action{
		{Kind: exchange.ActionPlaceMarket, Time: 100, OrderID: 7, Order: order, Signature: sig},
		{Kind: exchange.ActionCancel, Time: 101, OrderID: 9, Cancel: cancel, Signature: csig},
	} {
		got, err := fromRecord(toRecord(a))
		if err != nil {
			t.Fatalf("fromRecord: %v", err)
		}
		if !reflect.DeepEqual(got, a) {
			t.Fatalf("round trip:\ngot  %+v\nwant %+v", got, a)
		}
	}
}
