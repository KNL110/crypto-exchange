package matchengine

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled int64
	Price      float64
}