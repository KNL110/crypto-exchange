package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/labstack/echo/v5"

	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

func newTestExchange(t *testing.T) *exchange.Exchange {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	ex, err := exchange.NewExchange(hex.EncodeToString(crypto.FromECDSA(key)), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}
	return ex
}

func newTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	RegisterRoutes(e.Group("/api/v1/exchange"), NewHandler(newTestExchange(t)))
	return e
}

// get requests path and decodes a 200 JSON response into out.
func get(t *testing.T, e *echo.Echo, path string, out any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code == http.StatusOK && out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("GET %s: decode %s: %v", path, rec.Body, err)
		}
	}
	return rec.Code
}

func symbolsOf(books []orderBookResponse) []string {
	syms := make([]string, 0, len(books))
	for _, b := range books {
		syms = append(syms, b.Symbol)
	}
	return syms
}

func TestOrderBookBySymbolPath(t *testing.T) {
	e := newTestServer(t)

	var book orderBookResponse
	if code := get(t, e, "/api/v1/exchange/orderbooks/ETH", &book); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if book.Symbol != "ETH" {
		t.Fatalf("symbol = %q, want ETH", book.Symbol)
	}
	if code := get(t, e, "/api/v1/exchange/orderbooks/DOGE", nil); code != http.StatusNotFound {
		t.Fatalf("unknown symbol: status = %d, want 404", code)
	}
}

func TestOrderBooksForCommaSeparatedSymbols(t *testing.T) {
	e := newTestServer(t)

	var books []orderBookResponse
	if code := get(t, e, "/api/v1/exchange/orderbooks?symbols=BTC,ETH", &books); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got := symbolsOf(books); !reflect.DeepEqual(got, []string{"BTC", "ETH"}) {
		t.Fatalf("symbols = %v, want [BTC ETH] in the order asked", got)
	}

	// Blanks and duplicates are ignored.
	if code := get(t, e, "/api/v1/exchange/orderbooks?symbols=ETH,%20,ETH,", &books); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got := symbolsOf(books); !reflect.DeepEqual(got, []string{"ETH"}) {
		t.Fatalf("symbols = %v, want [ETH]", got)
	}

	if code := get(t, e, "/api/v1/exchange/orderbooks?symbols=ETH,DOGE", nil); code != http.StatusNotFound {
		t.Fatalf("one unknown symbol: status = %d, want 404", code)
	}
}

func TestOrderBooksWithoutSymbolsReturnsAll(t *testing.T) {
	e := newTestServer(t)

	var books []orderBookResponse
	if code := get(t, e, "/api/v1/exchange/orderbooks", &books); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	want := make([]string, 0, len(symbol.Configured))
	for _, s := range symbol.Configured {
		want = append(want, string(s))
	}
	if got := symbolsOf(books); !reflect.DeepEqual(got, want) {
		t.Fatalf("symbols = %v, want every configured symbol %v", got, want)
	}
}

func TestUnexpectedErrorUsesErrorShape(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

	h := NewHandler(newTestExchange(t))
	if err := h.mapError(c, errors.New("db exploded")); err != nil {
		t.Fatalf("mapError: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var got errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %s is not {\"error\": ...}: %v", rec.Body, err)
	}
	if got.Error != "internal error" {
		t.Fatalf("error = %q, want %q (internal details must not leak)", got.Error, "internal error")
	}
}
