package matchengine

import (
	"fmt"
	"sort"
)

// OrderBook holds all price levels for both sides of the market.
type OrderBook struct {
	asks      Limits
	bids      Limits
	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		asks:      Limits{},
		bids:      Limits{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.asks, ordBook.bids)
}

func (ordBook *OrderBook) Asks() Limits {
	sort.Sort(BybestAsk{ordBook.asks})
	return ordBook.asks
}

func (ordBook *OrderBook) Bids() Limits {
	sort.Sort(BybestBid{ordBook.bids})
	return ordBook.bids
}


func (ordBook *OrderBook) PlaceMarketOrder(order *Order) []MatchedOrder {
	matches := []MatchedOrder{}

	if order.IsBid {
		for _, askLimit := range ordBook.Asks() {
			matches = askLimit.FillOrder(order)
		}
	}
	
	return matches 
}


func (ordBook *OrderBook) PlaceLimitOrder(order *Order, price float64) {
	if order.IsBid {
		lim, exists := ordBook.BidLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.BidLimits[price] = lim
			ordBook.bids = append(ordBook.bids, lim)
		}
		lim.AddOrder(order)
	} else {
		lim, exists := ordBook.AskLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.AskLimits[price] = lim
			ordBook.asks = append(ordBook.asks, lim)
		}
		lim.AddOrder(order)
	}
}
