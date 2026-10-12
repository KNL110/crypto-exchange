package matchengine

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Order represents a single buy or sell order resting at a price level.
// Size is what is still unfilled; Filled is how much has traded so far.
type Order struct {
	ID        uint64
	Maker     common.Address
	Size      uint64
	Filled    uint64
	IsBid     bool
	Expiry    uint64 // unix seconds; 0 means it never expires
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

func (order *Order) isExpired(now uint64) bool {
	return order.Expiry != 0 && order.Expiry <= now
}
