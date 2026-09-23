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
	orders    map[uint64]*Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
		orders:    make(map[uint64]*Order),
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.Asks(), ordBook.Bids())
}

func (ordBook *OrderBook) Asks() []*Limit {
	sort.Sort(BybestAsk{ordBook.asks})
	return ordBook.asks
}

func (ordBook *OrderBook) Bids() Limits {
	sort.Sort(BybestBid{ordBook.bids})
	return ordBook.bids
}

func (ordBook *OrderBook) GetOrder(id uint64) (*Order, bool) {
	order, ok := ordBook.orders[id]
	return order, ok
}

func (ordBook *OrderBook) PlaceLimitOrder(order *Order, price float64) error {
	if order.ID != 0 {
		if _, exists := ordBook.orders[order.ID]; exists {
			return fmt.Errorf("PlaceLimitOrder -> %w", ErrDuplicateOrderID)
		}
	}

	var err error = nil
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
	if order.ID != 0 {
		ordBook.orders[order.ID] = order
	}
	return err
}

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

func (ordBook *OrderBook) PlaceMarketOrder(order *Order) ([]MatchedOrder, error) {
	matches := []MatchedOrder{}
	toDelete := []*Limit{}

	if order.IsBid {
		if order.Size > ordBook.AskTotalVolume() {
			return nil, fmt.Errorf("PlaceMarketOrder -> %w", ErrInsufficientLiquidity)
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
			return nil, fmt.Errorf("PlaceMarketOrder -> %w", ErrInsufficientLiquidity)
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
	for _, m := range matches {
		resting := m.Bid
		if order.IsBid {
			resting = m.Ask
		}
		if resting.IsFilled() {
			delete(ordBook.orders, resting.ID)
		}
	}
	if len(toDelete) > 0 {
		for _, lim := range toDelete {
			ordBook.removeLimit(lim, !order.IsBid)
		}
		ordBook.DeleteLimit(nil, !order.IsBid)
	}
	return matches, nil
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


func (ordBook *OrderBook) CancelOrder(id uint64) (*Order, error) {
	order, ok := ordBook.orders[id]
	if !ok {
		return nil, fmt.Errorf("CancelOrder -> %w", ErrOrderNotFound)
	}

	lim := order.Limit
	if err := lim.DeleteOrder(order); err != nil {
		return nil, fmt.Errorf("CancelOrder -> %w", err)
	}
	delete(ordBook.orders, id)

	if lim.IsEmpty() {
		ordBook.DeleteLimit(lim, order.IsBid)
	}
	return order, nil
}
