package matchengine

import (
	"fmt"
)


type Limit struct {
	Price       float64
	TotalVolume int64
	len         int64
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
	return fmt.Sprintf("Limit[Price: %.2f, TotalVolume: %d]", lim.Price, lim.TotalVolume)
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

//-------------------------------------------------------------------------------
type Limits []*Limit

func (l Limits) Len() int {
	return len(l)
}
func (l Limits) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type BybestAsk struct{ Limits }
type BybestBid struct{ Limits }

func (l BybestAsk) Less(i, j int) bool {
	return l.Limits[i].Price < l.Limits[j].Price
}

func (l BybestBid) Less(i, j int) bool {
	return l.Limits[i].Price > l.Limits[j].Price
}
