package exchange

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func newTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	RegisterRoutes(e.Group("/api/v1/exchange"), NewHandler(newTestExchange(t)))
	return e
}

func TestOrderBookFromQueryString(t *testing.T) {
	e := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange/orderbook?symbol=ETH", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got orderBookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Symbol != "ETH" {
		t.Fatalf("symbol = %q, want ETH", got.Symbol)
	}
}

func TestManyOrderBooksFromQueryString(t *testing.T) {
	e := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange/orderbooks?symbols=ETH&symbols=BTC", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got map[string]orderBookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got["ETH"].Symbol != "ETH" || got["BTC"].Symbol != "BTC" {
		t.Fatalf("books = %+v, want ETH and BTC", got)
	}
}

func TestOrderBookUnknownSymbolFromQueryString(t *testing.T) {
	e := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange/orderbook?symbol=DOGE", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body %s", rec.Code, rec.Body)
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
