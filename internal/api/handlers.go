// Package api is the exchange's HTTP layer: request/response DTOs, handlers
// that call the exchange service, routes, and error-to-status mapping.
package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

// Handler serves the exchange's HTTP endpoints.
type Handler struct {
	ex *exchange.Exchange
}

func NewHandler(ex *exchange.Exchange) *Handler {
	return &Handler{ex: ex}
}

func side(isbid bool) string {
	if isbid {
		return "buy"
	} else {
		return "sell"
	}
}

// mapError translates a domain or request-validation error into an HTTP
// status code and writes the corresponding JSON error response.
func (h *Handler) mapError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, exchange.ErrUnknownSymbol), errors.Is(err, exchange.ErrOrderNotFound):
		return c.JSON(http.StatusNotFound, newErrorResponse(err))
	case errors.Is(err, exchange.ErrInsufficientLiquidity):
		return c.JSON(http.StatusConflict, newErrorResponse(err))
	case errors.Is(err, exchange.ErrInvalidSize), errors.Is(err, exchange.ErrInvalidPrice),
		errors.Is(err, exchange.ErrOrderTypeMismatch), errors.Is(err, exchange.ErrOrderExpired):
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	case errors.Is(err, exchange.ErrInvalidSignature):
		return c.JSON(http.StatusUnauthorized, newErrorResponse(err))
	case errors.Is(err, exchange.ErrNotOrderOwner):
		return c.JSON(http.StatusForbidden, newErrorResponse(err))
	case errors.Is(err, exchange.ErrNonceUsed):
		return c.JSON(http.StatusConflict, newErrorResponse(err))
	default:
		// Don't leak internal details; keep the same {"error": ...} shape as every other error.
		return c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}

func (h *Handler) PlaceLimitOrder(c *echo.Context) error {
	var req signedOrderRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	}
	placed, err := h.ex.PlaceLimitOrder(req.message(), req.Signature)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(http.StatusCreated, newPlaceOrderResponse(placed, symbol.Symbol(req.Symbol)))
}

func (h *Handler) PlaceMarketOrder(c *echo.Context) error {
	var req signedOrderRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	}
	placed, err := h.ex.PlaceMarketOrder(req.message(), req.Signature)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(http.StatusCreated, newPlaceOrderResponse(placed, symbol.Symbol(req.Symbol)))
}

func (h *Handler) CancelOrder(c *echo.Context) error {
	var req cancelOrderRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	}
	order, err := h.ex.CancelOrder(req.message(), req.Signature)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(http.StatusOK, newOrderResponse(order, symbol.Symbol(req.Symbol)))
}

func (h *Handler) OrderBook(c *echo.Context) error {
	var req orderBookRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	}
	book, err := h.ex.OrderBook(symbol.Symbol(req.Symbol))
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(http.StatusOK, newOrderBookResponse(book))
}

func (h *Handler) ManyOrderBooks(c *echo.Context) error {
	var req ManyOrderBookRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(err))
	}

	orderBooks := make(map[string]orderBookResponse)
	for _, symStr := range req.Symbols {
		book, err := h.ex.OrderBook(symbol.Symbol(symStr))
		if err != nil {
			return h.mapError(c, err)
		}
		orderBooks[symStr] = newOrderBookResponse(book)
	}

	return c.JSON(http.StatusOK, orderBooks)
}

// Health reports server status along with the configured trading symbols.
func (h *Handler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, newHealthResponse("ok", h.ex.Symbols()))
}
