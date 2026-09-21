package matchengine

import (
	"fmt"
	"sort"
)

// OrderBook holds all price levels for both sides of the market.
type OrderBook struct {
	asks      []*Limit
	bids      []*Limit
	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.Asks(), ordBook.Bids()) //TODO:.Asks and .Bids are not methods, so this will not work. Need to implement a way to print the order book
}

func (ordBook *OrderBook) Asks() []*Limit {
	sort.Sort(BybestAsk{ordBook.asks})
	return ordBook.asks
}

func (ordBook *OrderBook) Bids() Limits {
	sort.Sort(BybestBid{ordBook.bids})
	return ordBook.bids
}

//	func DeleteNode(head *Limit, tail *Limit, lim *Limit) {
//		if lim.prev != nil {
//			lim.prev.next = lim.next
//		} else {
//			head = lim.next
//		}
//		if lim.next != nil {
//			lim.next.prev = lim.prev
//		} else {
//			tail = lim.prev
//		}
//	}
func (ordBook *OrderBook) removeLimit(lim *Limit, isBid bool) {
	if isBid {
		delete(ordBook.BidLimits, lim.Price)
		for i, l := range ordBook.bids {
			if l == lim {
				ordBook.bids[i], ordBook.bids[len(ordBook.bids)-1] = ordBook.bids[len(ordBook.bids)-1], nil
				ordBook.bids = ordBook.bids[:len(ordBook.bids)-1]
				break
			}
		}
	} else {
		delete(ordBook.AskLimits, lim.Price)
		for i, l := range ordBook.asks {
			if l == lim {
				ordBook.asks[i], ordBook.asks[len(ordBook.asks)-1] = ordBook.asks[len(ordBook.asks)-1], nil
				ordBook.asks = ordBook.asks[:len(ordBook.asks)-1]
				break
			}
		}
	}
}

func (ordBook *OrderBook) DeleteLimit(lim *Limit, isBid bool) {
	if lim != nil {
		ordBook.removeLimit(lim, isBid)
	}
	if isBid {
		sort.Sort(BybestBid{ordBook.bids})
	} else {
		sort.Sort(BybestAsk{ordBook.asks})
	}
}

func (ordBook *OrderBook) PlaceMarketOrder(order *Order) []MatchedOrder {
	matches := []MatchedOrder{}
	toDelete := []*Limit{}

	if order.IsBid {
		if order.Size > ordBook.AskTotalVolume() {
			panic("order size exceeds total ask volume")
		}
		for _, askLimit := range ordBook.Asks() {
			askLimit.FillOrder(order, &matches)

			if askLimit.IsEmpty() {
				toDelete = append(toDelete, askLimit)
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
				toDelete = append(toDelete, bidLimit)
			}
			if order.IsFilled() {
				break
			}
		}
	}
	if len(toDelete) > 0 {
		for _, lim := range toDelete {
			ordBook.removeLimit(lim, !order.IsBid)
		}
		ordBook.DeleteLimit(nil, !order.IsBid)
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

func (ordBook *OrderBook) BidTotalVolume() int64 {
	var total int64 = 0
	for _, lim := range ordBook.bids {
		total += lim.TotalVolume
	}
	return total
}

func (ordBook *OrderBook) AskTotalVolume() int64 {
	var total int64 = 0
	for _, lim := range ordBook.asks {
		total += lim.TotalVolume
	}
	return total
}

func (ordBook *OrderBook) cancelOrder(order *Order, price float64) {
	//TODO: implement cancelOrder logic
}

//
