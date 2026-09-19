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

func (ordBook *OrderBook) DeleteLimit(lim *Limit, isBid bool) {
	if isBid {
		delete(ordBook.BidLimits, lim.Price)
		for i, l := range ordBook.bids {
			if l == lim {
				ordBook.bids[i] = ordBook.bids[len(ordBook.bids)-1]
				ordBook.bids = ordBook.bids[:len(ordBook.bids)-1]
				break
			}
		}
		ordBook.bids = ordBook.Bids() // re-sort bids after deletion
	} else {
		delete(ordBook.AskLimits, lim.Price)
		for i, l := range ordBook.asks {
			if l == lim {
				ordBook.asks[i] = ordBook.asks[len(ordBook.asks)-1]
				ordBook.asks = ordBook.asks[:len(ordBook.asks)-1]
				break
			}
		}
		ordBook.asks = ordBook.Asks() // re-sort asks after deletion
	}
}


// clearEmptiedLimits removes drained limits from the book and re-sorts
func (ordBook *OrderBook) clearEmptiedLimits(limits []*Limit, isBid bool) {
	for _, lim := range limits {
		ordBook.DeleteLimit(lim, isBid)
	}
}

func (ordBook *OrderBook) PlaceMarketOrder(order *Order) []MatchedOrder {
	matches := []MatchedOrder{}
	emptied := Limits{}

	if order.IsBid {
		if order.Size > ordBook.AskTotalVolume() {
			panic("order size exceeds total ask volume")
		}
		for _, askLimit := range ordBook.Asks() {
			askLimit.FillOrder(order, &matches)

			if askLimit.IsEmpty() {
				emptied = append(emptied, askLimit)
			}
			if order.IsFilled() {
				break
			}
		}
	} else {
		if order.Size > ordBook.BidTotalVolume() {
			panic("order size exceeds total bid volume")
		}
		for _, bidLimit := range ordBook.Bids() {
			bidLimit.FillOrder(order, &matches)

			if bidLimit.IsEmpty() {
				emptied = append(emptied, bidLimit)
			}
			if order.IsFilled() {
				break
			}
		}
	}

	if len(emptied) > 0 {
		// order.IsBid consumed asks, so need to clear emptied asks, and vice versa
		ordBook.clearEmptiedLimits(emptied, !order.IsBid)
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

func (ordBook *OrderBook) BidTotalVolume() float64 {
	total := 0.0
	for _, lim := range ordBook.bids {
		total += lim.TotalVolume
	}
	return total
}

func (ordBook *OrderBook) AskTotalVolume() float64 {
	total := 0.0
	for _, lim := range ordBook.asks {
		total += lim.TotalVolume
	}
	return total
}
