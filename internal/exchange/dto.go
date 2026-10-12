package exchange

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
)

// signedOrderRequest is an EIP-712 signed order (see signing.go). Limit and
// market orders share it; a market order is signed with price 0.
type signedOrderRequest struct {
	Maker     common.Address `json:"maker"`
	Symbol    string         `json:"symbol"`
	IsBid     bool           `json:"isBid"`
	Size      uint64         `json:"size"`
	Price     uint64         `json:"price"`
	Nonce     uint64         `json:"nonce"`
	Expiry    uint64         `json:"expiry"`
	Signature hexutil.Bytes  `json:"signature"`
}

func (r signedOrderRequest) message() OrderMessage {
	return OrderMessage{
		Maker:  r.Maker,
		Symbol: Symbol(r.Symbol),
		IsBid:  r.IsBid,
		Size:   r.Size,
		Price:  r.Price,
		Nonce:  r.Nonce,
		Expiry: r.Expiry,
	}
}

type cancelOrderRequest struct {
	Maker     common.Address `json:"maker"`
	Symbol    string         `json:"symbol"`
	OrderID   uint64         `json:"orderId"`
	Signature hexutil.Bytes  `json:"signature"`
}

func (r cancelOrderRequest) message() CancelMessage {
	return CancelMessage{
		Maker:   r.Maker,
		Symbol:  Symbol(r.Symbol),
		OrderID: r.OrderID,
	}
}

type orderBookRequest struct {
	Symbol string `json:"symbol"`
}

type ManyOrderBookRequest struct {
	Symbols []string `json:"symbols"`
}

type orderResponse struct {
	Id     uint64  `json:"id"`
	Maker  string  `json:"maker"`
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Size   uint64   `json:"size"`
	Price  float64 `json:"price,omitempty"`
}

func newOrderResponse(order *matchengine.Order, symbol Symbol) orderResponse {
	res := orderResponse{
		Id:     order.ID,
		Maker:  order.Maker.Hex(),
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
	SizeFilled uint64   `json:"sizeFilled"`
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

type limitResponse struct {
	Price        float64         `json:"price"`
	TotalVolumne uint64           `json:"totalVolume"`
	TotalOrders  uint64           `json:"totalOrders"`
	Orders       []orderResponse `json:"orders"`
}

func newLimitResponse(lim *matchengine.Limit, symbol Symbol) limitResponse {
	res := limitResponse{
		Price:        lim.Price,
		TotalVolumne: lim.TotalVolume,
		TotalOrders:  lim.Len,
		Orders:       make([]orderResponse, 0, lim.Len),
	}
	orders := lim.Orders()
	for i := range orders {
		res.Orders = append(res.Orders, newOrderResponse(orders[i], symbol))
	}
	return res
}

type orderBookResponse struct {
	Symbol string          `json:"symbol"`
	Bids   []limitResponse `json:"bids"`
	Asks   []limitResponse `json:"asks"`
}

func newOrderBookResponse(book *matchengine.OrderBook, symbol Symbol) orderBookResponse {
	res := orderBookResponse{
		Symbol: string(symbol),
		Bids:   make([]limitResponse, 0, len(book.BidLimits)),
		Asks:   make([]limitResponse, 0, len(book.AskLimits)),
	}
	for _, lim := range book.Bids() {
		res.Bids = append(res.Bids, newLimitResponse(lim, symbol))
	}
	for _, lim := range book.Asks() {
		res.Asks = append(res.Asks, newLimitResponse(lim, symbol))
	}
	return res
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
