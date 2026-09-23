package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v5"
	"net/http"
)

type Handler struct {
	ex *Exchange
}

func side(isbid bool) string{
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
	case errors.Is(err, ErrUnknownSymbol), errors.Is(err, ErrOrderNotFound):
		return c.JSON(http.StatusNotFound, ErrorResponse{err.Error()})
	case errors.Is(err, ErrInsufficientLiquidity):
		return c.JSON(http.StatusConflict, ErrorResponse{err.Error()})
	case errors.Is(err, ErrInvalidSize), errors.Is(err, ErrInvalidPrice):
		return c.JSON(http.StatusBadRequest, ErrorResponse{err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) PlaceLimitOrder(c *echo.Context) error {
	var req placeLimitOrderRequest

	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return fmt.Errorf("PlaceLimitOrderHandler -> %w", err)
	}
	symbol := Symbol(req.Symbol)

	order, err := h.ex.placeLimitOrder(req.IsBid, req.Price, uint64(req.Size), symbol)
	if err != nil {
		return h.mapError(c,err)
	}

	return c.JSON(http.StatusCreated,OrderResponse{
		Symbol: string(symbol),
		Side: side(order.IsBid),
		Size: order.Size,
		Price: order.Limit.Price,
	})
}
