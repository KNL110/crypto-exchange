package matchengine

import (
	"fmt"
	"time"
)

// Order represents a single buy or sell order resting at a price level.
type Order struct {
	ID        uint64
	Size      int64
	IsBid     bool
	Limit     *Limit
	Timestamp int64

	prev *Order
	next *Order
}

func NewOrder(id uint64,isbid bool, size int64) *Order {
	return &Order{
		ID:        id,
		Size:      size,
		IsBid:     isbid,
		Limit:     nil,
		Timestamp: time.Now().UnixNano(),
		prev:      nil,
		next:      nil,
	}
}

func (order *Order) String() string {
	return fmt.Sprintf("Order[size: %d]", order.Size)
}

func (order *Order) IsFilled() bool {
	return order.Size == 0
}
