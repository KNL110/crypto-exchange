package api

import (
	"github.com/labstack/echo/v5"
)

func RegisterRoutes(g *echo.Group, h *Handler) {

	g.GET("/orderbooks", h.OrderBooks)
	g.GET("/orderbooks/:symbol", h.OrderBook)
	g.POST("/orders/limit", h.PlaceLimitOrder)
	g.POST("/orders/market", h.PlaceMarketOrder)
	g.DELETE("/orders/cancel", h.CancelOrder)
}
