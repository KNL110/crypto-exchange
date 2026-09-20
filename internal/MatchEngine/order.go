package matchengine

import (
	"fmt"
	"time"
)

// Order represents a single buy or sell order resting at a price level.
type Order struct {
	Size      float64
	IsBid     bool
	Limit     *Limit
	Timestamp int64

	prev *Order
	next *Order
}

func NewOrder(isbid bool, size float64) *Order {
	return &Order{
		Size:      size,
		IsBid:     isbid,
		Timestamp: time.Now().UnixNano(),
		prev:      nil,
		next:      nil,
	}
}

func (order *Order) String() string {
	return fmt.Sprintf("Order[size: %.2f , isBid: %t]", order.Size, order.IsBid)
}

func (order *Order) IsFilled() bool {
	return order.Size == 0
}

// Orders implements sort.Interface, ordering orders FIFO by arrival time.
type Orders []*Order

func (ord Orders) Len() int {
	return len(ord)
}

func (ord Orders) Swap(i, j int) {
	ord[i], ord[j] = ord[j], ord[i]
}

func (ord Orders) Less(i, j int) bool {
	return ord[i].Timestamp < ord[j].Timestamp
}
