package matchengine

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled uint64
	Price      float64
}