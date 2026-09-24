package exchange

import matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"

type placeLimitOrderRequest struct {
	Symbol string  `json:"symbol"`
	IsBid  bool    `json:"isBid"`
	Size   int64   `json:"size"`
	Price  float64 `json:"price"`
}

type placeMarketOrderRequest struct {
	Symbol string `json:"symbol"`
	IsBid  bool   `json:"isBid"`
	Size   int64  `json:"size"`
}

type cancelOrderRequest struct {
	Symbol  string `json:"symbol"`
	OrderID uint64 `json:"orderId"`
}

type orderResponse struct {
	Id     uint64  `json:"id"`
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Size   int64   `json:"size"`
	Price  float64 `json:"price,omitempty"`
}

func newOrderResponse(order *matchengine.Order, symbol Symbol) orderResponse {
	res := orderResponse{
		Id:     order.ID,
		Symbol: string(symbol),
		Side:   side(order.IsBid),
		Size:   order.Size,
	}
	// market orders never rest on a limit, so they have no price
	if order.Limit != nil {
		res.Price = order.Limit.Price
	}
	return res
}

type placeMarketOrderResponse struct {
	Order   orderResponse          `json:"order"`
	Matches []matchedOrderResponse `json:"matches"`
}

type matchedOrderResponse struct {
	SizeFilled int64   `json:"sizeFilled"`
	Price      float64 `json:"price"`
}

func newMatchedOrderResponse(m matchengine.MatchedOrder) matchedOrderResponse {
	return matchedOrderResponse{
		SizeFilled: m.SizeFilled,
		Price:      m.Price,
	}
}

func newMarketOrderResponse(order *matchengine.Order, symbol Symbol, matches *[]matchengine.MatchedOrder) placeMarketOrderResponse {
	res := placeMarketOrderResponse{
		Order:   newOrderResponse(order, symbol),
		Matches: make([]matchedOrderResponse, 0, len(*matches)),
	}
	for i := range *matches {
		res.Matches = append(res.Matches, newMatchedOrderResponse((*matches)[i]))
	}
	return res
}

type errorResponse struct {
	Error string `json:"error"`
}

func newErrorResponse(err error) errorResponse {
	return errorResponse{Error: err.Error()}
}

type healthResponse struct {
	Status  string   `json:"status"`
	Symbols []string `json:"symbols"`
}

func newHealthResponse(status string, symbols []Symbol) healthResponse {
	res := healthResponse{
		Status:  status,
		Symbols: make([]string, 0, len(symbols)),
	}
	for _, s := range symbols {
		res.Symbols = append(res.Symbols, string(s))
	}
	return res
}
