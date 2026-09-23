package exchange

type placeLimitOrderRequest struct {
	Symbol string  `json:"symbol"`
	IsBid  bool    `json:"is_bid"`
	Size   int64   `json:"size"`
	Price  float64 `json:"price"`
}

type placeMarketOrderRequest struct {
	Symbol string `json:"symbol"`
	IsBid  bool   `json:"is_bid"`
	Size   int64  `json:"size"`
}

type OrderResponse struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Size      int64   `json:"size"`
	Price     float64 `json:"price,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Status  string   `json:"status"`
	Symbols []string `json:"symbols"`
}