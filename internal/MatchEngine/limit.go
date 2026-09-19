package matchengine

import (
	"fmt"
	"sort"
)

// Limit represents a single price level and the orders resting at it.
type Limit struct {
	Price float64
	Orders
	TotalVolume float64
}

func NewLimit(price float64) *Limit {
	return &Limit{
		Price:  price,
		Orders: []*Order{},
	}
}

func (lim *Limit) String() string {
	return fmt.Sprintf("Limit[Price: %.2f, TotalVolume: %.2f, Orders: %v]", lim.Price, lim.TotalVolume, lim.Orders)
}

func (lim *Limit) IsEmpty() bool {
	return len(lim.Orders) == 0
}

func (lim *Limit) AddOrder(order *Order) {
	order.Limit = lim
	lim.Orders = append(lim.Orders, order)
	lim.TotalVolume += order.Size
}

// removeOrder detaches order from lim without re-sorting, so callers that
// remove several orders in a row (e.g. FillOrder) can batch the resort.
func (lim *Limit) removeOrder(order *Order) {
	ordLen := len(lim.Orders)

	for i, o := range lim.Orders {
		if o == order {
			lim.Orders[i] = lim.Orders[ordLen-1]
			lim.Orders[ordLen-1] = nil //avoid memory leak
			lim.Orders = lim.Orders[:ordLen-1]
			break
		}
	}
	order.Limit = nil //remove the limit reference from order since it is no longer in limit
	lim.TotalVolume -= order.Size
}

func (lim *Limit) DeleteOrder(order *Order) {
	lim.removeOrder(order)

	//sort to maintain FIFO order
	sort.Sort(lim.Orders)
}



func (lim *Limit) fillOrder(ordIncoming,ordResting *Order) MatchedOrder {
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

	for _, o := range lim.Orders {
		matched := lim.fillOrder(order, o)
		*matches = append(*matches, matched)

		if order.IsFilled() {
			break
		}
	}
}

type Limits []*Limit

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
