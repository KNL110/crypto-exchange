package matchengine

import "fmt"

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled float64
	Price      float64
}

// OrderBook holds all price levels for both sides of the market.
type OrderBook struct {
	Asks      Limits
	Bids      Limits
	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Asks:      Limits{},
		Bids:      Limits{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.Asks, ordBook.Bids)
}

// PlaceOrder matches the order against the book; if it is not fully
// filled, the remainder is added to the book.
func (ordBook *OrderBook) PlaceOrder(order *Order, price float64) []MatchedOrder {
	//matching logic:

	if order.Size > 0 {
		ordBook.addOrder(order, price)
	}

	return []MatchedOrder{}
}

// addOrder adds order to the matching limit, creating a new limit and
// adding it to the book if one does not already exist at that price.
func (ordBook *OrderBook) addOrder(order *Order, price float64) {
	if order.IsBid {
		lim, exists := ordBook.BidLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.BidLimits[price] = lim
			ordBook.Bids = append(ordBook.Bids, lim)
		}
		lim.AddOrder(order)
	} else {
		lim, exists := ordBook.AskLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.AskLimits[price] = lim
			ordBook.Asks = append(ordBook.Asks, lim)
		}
		lim.AddOrder(order)
	}
}
