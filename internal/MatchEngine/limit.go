package matchengine

import (
	"fmt"
)

// Limit represents a single price level and the orders resting at it.
type Limits []*Limit

type Limit struct {
	Price       float64
	TotalVolume float64
	len         uint64
	head        *Order
	tail        *Order
}

func NewLimit(price float64) *Limit {
	return &Limit{
		Price:       price,
		TotalVolume: 0,
		head:        nil,
		tail:        nil,
		len:         0,
	}
}

func (lim *Limit) String() string {
	return fmt.Sprintf("Limit[Price: %.2f, TotalVolume: %.2f]", lim.Price, lim.TotalVolume)
}

func (lim *Limit) IsEmpty() bool {
	return (lim.TotalVolume == 0)
}

func (lim *Limit) AddOrder(order *Order) {
	order.Limit = lim

	if lim.head == nil {
		lim.head = order
		lim.tail = order
	} else {
		lim.tail.next = order
		order.prev = lim.tail
		lim.tail = order
	}
	lim.len++
	lim.TotalVolume += order.Size
}

func (lim *Limit) DeleteOrder(order *Order) {
	if order.Limit != lim {
		panic("order does not belong to this limit")
	}

	if order.prev != nil {
		order.prev.next = order.next
	} else {
		lim.head = order.next
	}

	if order.next != nil {
		order.next.prev = order.prev
	} else {
		lim.tail = order.prev
	}

	order.prev = nil
	order.next = nil

	lim.len--
	lim.TotalVolume -= order.Size
}

func (lim *Limit) fillOrder(ordIncoming, ordResting *Order) MatchedOrder {
	var matched MatchedOrder

	if ordIncoming.IsBid {
		matched.Bid = ordIncoming
		matched.Ask = ordResting
	} else {
		matched.Bid = ordResting
		matched.Ask = ordIncoming
	}

	if ordIncoming.Size >= ordResting.Size {
		matched.SizeFilled = ordResting.Size
		matched.Price = lim.Price
		ordIncoming.Size -= ordResting.Size
		ordResting.Size = 0
	} else {
		matched.SizeFilled = ordIncoming.Size
		matched.Price = lim.Price
		ordResting.Size -= ordIncoming.Size
		ordIncoming.Size = 0
	}

	lim.TotalVolume -= matched.SizeFilled

	return matched
}

func (lim *Limit) FillOrder(order *Order, matches *[]MatchedOrder) {

	for o := lim.head; o != nil; {
		next := o.next
		matched := lim.fillOrder(order, o)
		*matches = append(*matches, matched)

		if o.IsFilled() {
			lim.DeleteOrder(o)
		}
		if order.IsFilled() {
			break
		}
		o = next
	}
}

func (lims Limits) Len() int {
	return len(lims)
}

func (lims Limits) Swap(i, j int) {
	lims[i], lims[j] = lims[j], lims[i]
}

// BybestAsk sorts Limits ascending by price (lowest ask first).
type BybestAsk struct {
	Limits
}

func (a BybestAsk) Less(i, j int) bool {
	return a.Limits[i].Price < a.Limits[j].Price
}

// BybestBid sorts Limits descending by price (highest bid first).
type BybestBid struct {
	Limits
}

func (b BybestBid) Less(i, j int) bool {
	return b.Limits[i].Price > b.Limits[j].Price
}
