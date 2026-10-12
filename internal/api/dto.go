package api

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"

	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

// signedOrderRequest is an EIP-712 signed order (see signing.go). Limit and
// market orders share it; for a market order, price is the worst price accepted.
type signedOrderRequest struct {
	Maker     common.Address `json:"maker"`
	Symbol    string         `json:"symbol"`
	IsBid     bool           `json:"isBid"`
	IsMarket  bool           `json:"isMarket"`
	Size      uint64         `json:"size"`
	Price     uint64         `json:"price"`
	Nonce     uint64         `json:"nonce"`
	Expiry    uint64         `json:"expiry"`
	Signature hexutil.Bytes  `json:"signature"`
}

func (r signedOrderRequest) message() signing.Order {
	return signing.Order{
		Maker:    r.Maker,
		Symbol:   symbol.Symbol(r.Symbol),
		IsBid:    r.IsBid,
		IsMarket: r.IsMarket,
		Size:     r.Size,
		Price:    r.Price,
		Nonce:    r.Nonce,
		Expiry:   r.Expiry,
	}
}

type cancelOrderRequest struct {
	Maker     common.Address `json:"maker"`
	Symbol    string         `json:"symbol"`
	OrderID   uint64         `json:"orderId"`
	Signature hexutil.Bytes  `json:"signature"`
}

func (r cancelOrderRequest) message() signing.Cancel {
	return signing.Cancel{
		Maker:   r.Maker,
		Symbol:  symbol.Symbol(r.Symbol),
		OrderID: r.OrderID,
	}
}

// orderBookRequest and ManyOrderBookRequest are read from the query string
// (?symbol=ETH, ?symbols=ETH&symbols=BTC): browsers can't send a body with GET.
type orderBookRequest struct {
	Symbol string `query:"symbol" json:"symbol"`
}

type ManyOrderBookRequest struct {
	Symbols []string `query:"symbols" json:"symbols"`
}

type orderResponse struct {
	Id     uint64 `json:"id"`
	Maker  string `json:"maker"`
	Symbol string `json:"symbol"`
	Side   string `json:"side"`
	Size   uint64 `json:"size"`
	Filled uint64 `json:"filled"`
	Price  uint64 `json:"price,omitempty"`
}

func newOrderResponse(order *matchengine.Order, symbol symbol.Symbol) orderResponse {
	res := orderResponse{
		Id:     order.ID,
		Maker:  order.Maker.Hex(),
		Symbol: string(symbol),
		Side:   side(order.IsBid),
		Size:   order.Size,
		Filled: order.Filled,
	}
	// only a resting order sits on a limit, so filled and market orders have no price
	if order.Limit != nil {
		res.Price = order.Limit.Price
	}
	return res
}

// placeOrderResponse is returned for both limit and market orders: the order
// after matching, every fill it made, and the maker's own resting orders that
// self-trade prevention cancelled.
type placeOrderResponse struct {
	Order     orderResponse          `json:"order"`
	Matches   []matchedOrderResponse `json:"matches"`
	Cancelled []orderResponse        `json:"cancelled"`
}

type matchedOrderResponse struct {
	SizeFilled uint64 `json:"sizeFilled"`
	Price      uint64 `json:"price"`
}

func newMatchedOrderResponse(m matchengine.MatchedOrder) matchedOrderResponse {
	return matchedOrderResponse{
		SizeFilled: m.SizeFilled,
		Price:      m.Price,
	}
}

func newPlaceOrderResponse(placed exchange.Placed, sym symbol.Symbol) placeOrderResponse {
	res := placeOrderResponse{
		Order:     newOrderResponse(placed.Order, sym),
		Matches:   make([]matchedOrderResponse, 0, len(placed.Matches)),
		Cancelled: make([]orderResponse, 0, len(placed.Cancelled)),
	}
	for _, m := range placed.Matches {
		res.Matches = append(res.Matches, newMatchedOrderResponse(m))
	}
	for _, o := range placed.Cancelled {
		res.Cancelled = append(res.Cancelled, newOrderResponse(o, sym))
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
	Price        uint64          `json:"price"`
	TotalVolumne uint64          `json:"totalVolume"`
	TotalOrders  uint64          `json:"totalOrders"`
	Orders       []orderResponse `json:"orders"`
}

func newLimitResponse(level exchange.LevelSnapshot, sym symbol.Symbol) limitResponse {
	res := limitResponse{
		Price:        level.Price,
		TotalVolumne: level.TotalVolume,
		TotalOrders:  uint64(len(level.Orders)),
		Orders:       make([]orderResponse, 0, len(level.Orders)),
	}
	for _, o := range level.Orders {
		res.Orders = append(res.Orders, orderResponse{
			Id:     o.ID,
			Maker:  o.Maker.Hex(),
			Symbol: string(sym),
			Side:   side(o.IsBid),
			Size:   o.Size,
			Filled: o.Filled,
			Price:  level.Price,
		})
	}
	return res
}

type orderBookResponse struct {
	Symbol string          `json:"symbol"`
	Bids   []limitResponse `json:"bids"`
	Asks   []limitResponse `json:"asks"`
}

func newOrderBookResponse(book exchange.BookSnapshot) orderBookResponse {
	res := orderBookResponse{
		Symbol: string(book.Symbol),
		Bids:   make([]limitResponse, 0, len(book.Bids)),
		Asks:   make([]limitResponse, 0, len(book.Asks)),
	}
	for _, level := range book.Bids {
		res.Bids = append(res.Bids, newLimitResponse(level, book.Symbol))
	}
	for _, level := range book.Asks {
		res.Asks = append(res.Asks, newLimitResponse(level, book.Symbol))
	}
	return res
}

type healthResponse struct {
	Status  string   `json:"status"`
	Symbols []string `json:"symbols"`
}

func newHealthResponse(status string, symbols []symbol.Symbol) healthResponse {
	res := healthResponse{
		Status:  status,
		Symbols: make([]string, 0, len(symbols)),
	}
	for _, s := range symbols {
		res.Symbols = append(res.Symbols, string(s))
	}
	return res
}
