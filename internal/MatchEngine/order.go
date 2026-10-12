package matchengine

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Order represents a single buy or sell order resting at a price level.
type Order struct {
	ID        uint64
	Maker     common.Address
	Size      uint64
	IsBid     bool
	Limit     *Limit
	Timestamp int64

	prev *Order
	next *Order
}

func NewOrder(id uint64, isbid bool, size uint64, maker common.Address) *Order {
	return &Order{
		ID:        id,
		Maker:     maker,
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
