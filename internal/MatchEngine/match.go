package matchengine

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled float64
	Price      float64
}