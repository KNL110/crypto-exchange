package matchengine

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func assert(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v != %+v", a, b)
	}
}

func TestLimit(t *testing.T) {
	l := NewLimit(10_000)
	buyOrderA := NewOrder(1,true, 5)
	buyOrderB := NewOrder(2,true, 10)
	buyOrderC := NewOrder(3,true, 15)
	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	fmt.Println(l)

	l.DeleteOrder(buyOrderA)
	fmt.Println(l)

	l.DeleteOrder(buyOrderB)
	fmt.Println(l)

	l.DeleteOrder(buyOrderC)
	fmt.Println(l)

	buyOrder := NewOrder(4,true, 10)

	l.AddOrder(buyOrder)
	fmt.Println(l)
}

func TestPlaceLimitOrder(t *testing.T) {
	ordBook := NewOrderBook()

	sellOrderA := NewOrder(5,false, 10)
	sellOrderB := NewOrder(6,false, 5)
	ordBook.PlaceLimitOrder(sellOrderA, 10_000)
	ordBook.PlaceLimitOrder(sellOrderB, 8_000)

	assert(t, len(ordBook.asks), 2)
	for i := 0; i < len(ordBook.asks); i++ {
		fmt.Println(ordBook.asks[i])
	}
}

func TestPlaceMarketOrder(t *testing.T) {
	ordBook := NewOrderBook()

	// liquidity: two ask limits at different prices
	sellOrderA := NewOrder(7,false, 20)
	ordBook.PlaceLimitOrder(sellOrderA, 10_000)

	buyOrderA := NewOrder(8,true, 10)
	matches, err := ordBook.PlaceMarketOrder(buyOrderA)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}

	assert(t, len(matches), 1)
	assert(t, len(ordBook.asks), 1)
	assert(t, ordBook.AskTotalVolume(), int64(10))
	assert(t, matches[0].Price, 10_000.0)
	assert(t, matches[0].SizeFilled, int64(10))
	assert(t, buyOrderA.IsFilled(), true)

	for _, match := range matches {
		fmt.Printf("%+v", match)
	}

}

func TestPlaceMarketOrderWithMultipleMatches(t *testing.T) {
	ordBook := NewOrderBook()

	buyOrderA := NewOrder(9,true, 15)
	buyOrderB := NewOrder(10,true, 10)
	buyOrderC := NewOrder(11,true, 5)
	buyOrderD := NewOrder(12,true, 1)

	ordBook.PlaceLimitOrder(buyOrderC, 8_000)
	ordBook.PlaceLimitOrder(buyOrderB, 9_000)
	ordBook.PlaceLimitOrder(buyOrderD, 9_000)
	ordBook.PlaceLimitOrder(buyOrderA, 10_000)

	assert(t, ordBook.BidTotalVolume(), int64(31))

	sellOrder := NewOrder(13,false, 27)
	matches, err := ordBook.PlaceMarketOrder(sellOrder)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}

	assert(t, len(matches), 4)
	assert(t, ordBook.BidTotalVolume(), int64(4))
	assert(t, len(ordBook.bids), 1)

	for _, match := range matches {
		fmt.Printf("%+v\n", match)
	}

}

func TestOrderLookupBy(t *testing.T) {
	ordBook := NewOrderBook()

	order := NewOrder(42, true, 10)
	ordBook.PlaceLimitOrder(order, 9_000)

	got, ok := ordBook.GetOrder(42)
	assert(t, ok, true)
	assert(t, got, order)

	_, ok = ordBook.GetOrder(999)
	assert(t, ok, false)
}

func TestCancelOrder(t *testing.T) {
	ordBook := NewOrderBook()

	orderA := NewOrder(1, true, 10)
	orderB := NewOrder(2, true, 5)
	orderc := NewOrder(2, true, 10) // duplicate ID to test error handling
	ordBook.PlaceLimitOrder(orderA, 9_000)
	ordBook.PlaceLimitOrder(orderB, 9_000)
	err := ordBook.PlaceLimitOrder(orderc, 9_000)
	if err ==nil || !errors.Is(err, ErrDuplicateOrderID) {
		t.Fatalf("PlaceLimitOrder: err = %v want err dup", err)
	}
	assert(t, ordBook.BidTotalVolume(), int64(15))

	cancelled, err := ordBook.CancelOrder(orderA.ID)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	assert(t, cancelled, orderA)
	assert(t, ordBook.BidTotalVolume(), int64(5))
	_, ok := ordBook.GetOrder(1)
	assert(t, ok, false)

	// Cancelling the last order at a price level removes the limit entirely.
	if _, err := ordBook.CancelOrder(orderB.ID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	assert(t, len(ordBook.bids), 0)
	_, exists := ordBook.BidLimits[9_000]
	assert(t, exists, false)

	_, err = ordBook.CancelOrder(orderc.ID)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("CancelOrder err = %v,", err)
	}
}

func TestPlaceLimitOrderRejectsDuplicateID(t *testing.T) {
	ordBook := NewOrderBook()

	orderA := NewOrder(7, true, 10)
	if err := ordBook.PlaceLimitOrder(orderA, 9_000); err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}

	orderB := NewOrder(7, true, 5)
	err := ordBook.PlaceLimitOrder(orderB, 9_500)
	if !errors.Is(err, ErrDuplicateOrderID) {
		t.Fatalf("PlaceLimitOrder (duplicate id): err = %v, want ErrDuplicateOrderID", err)
	}

	// The book must be unchanged: the duplicate was rejected before any
	// mutation, not partially applied.
	assert(t, ordBook.BidTotalVolume(), int64(10))
	assert(t, len(ordBook.bids), 1)
}
func TestPlaceMarketOrderInsufficientLiquidity(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 5), 10_000)

	_, err := ordBook.PlaceMarketOrder(NewOrder(2, true, 10))
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}
	assert(t, ordBook.AskTotalVolume(), int64(5))
}
