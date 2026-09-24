package exchange

import (
	"github.com/labstack/echo/v5"
)

func RegisterRoutes(g *echo.Group, h *Handler) {
	
	g.GET("/orderbook", h.OrderBook)
	g.GET("/orderbooks", h.ManyOrderBooks)
	g.POST("/orders/limit", h.PlaceLimitOrder)
	g.POST("/orders/market", h.PlaceMarketOrder)
	g.DELETE("/orders/cancel", h.CancelOrder)
}
