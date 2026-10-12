package matchengine

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled uint64
	Price      uint64 // the resting order's price, in ticks
}