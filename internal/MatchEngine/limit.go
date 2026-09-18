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

func (lim *Limit) AddOrder(order *Order) {
	order.Limit = lim
	lim.Orders = append(lim.Orders, order)
	lim.TotalVolume += order.Size
}

func (lim *Limit) DeleteOrder(order *Order) {
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

	//sort to maintain FIFO order
	sort.Sort(lim.Orders)
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
