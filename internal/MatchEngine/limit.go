package matchengine

import (
	"fmt"
)

// Limit is one price level. Price is a whole number of ticks, never a float.
type Limit struct {
	Price       uint64
	TotalVolume uint64
	Len         uint64
	head        *Order
	tail        *Order
}

func NewLimit(price uint64) *Limit {
	return &Limit{
		Price:       price,
		TotalVolume: 0,
		head:        nil,
		tail:        nil,
		Len:         0,
	}
}

func (lim *Limit) String() string {
	return fmt.Sprintf("Limit[Price: %d, TotalVolume: %d]", lim.Price, lim.TotalVolume)
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

	lim.Len++
	lim.TotalVolume += order.Size
}

func (lim *Limit) Orders() []*Order { //TODO: optimize 
	orders := make([]*Order, 0, lim.Len)
	for o := lim.head; o != nil; o = o.next {
		orders = append(orders, o)
	}
	return orders
}

func (lim *Limit) DeleteOrder(order *Order) error {
	if order == nil {
		return fmt.Errorf("DeleteOrder -> %w", ErrNilOrder)
	}

	if order.Limit != lim {
		return fmt.Errorf("DeleteOrder -> %w",ErrWrongLimit)
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

	order.prev = nil  //TODO: consider for performance affects due to garbage collection
	order.next = nil

	lim.Len--
	lim.TotalVolume -= order.Size
	return nil
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

	matched.SizeFilled = min(ordIncoming.Size, ordResting.Size)
	matched.Price = lim.Price
	ordIncoming.Size -= matched.SizeFilled
	ordResting.Size -= matched.SizeFilled
	ordIncoming.Filled += matched.SizeFilled
	ordResting.Filled += matched.SizeFilled

	lim.TotalVolume -= matched.SizeFilled

	return matched
}

// FillOrder fills order against this level's resting orders, oldest first.
// Self-trade prevention: a resting order from the same maker is never traded
// against; it is removed from the level and appended to cancelled instead.
func (lim *Limit) FillOrder(order *Order, matches *[]MatchedOrder, cancelled *[]*Order) {

	for o := lim.head; o != nil; {
		next := o.next
		if o.Maker == order.Maker {
			lim.DeleteOrder(o)
			*cancelled = append(*cancelled, o)
			o = next
			continue
		}
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

// -------------------------------------------------------------------------------
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
