package matchengine

import (
	"fmt"
	"reflect"
	"testing"
)

func assert(t *testing.T, a,b any){
	if !reflect.DeepEqual(a,b){
		t.Errorf("%+v != %+v",a,b)
	}
}

// func TestLimit(t *testing.T) {
// 	l := NewLimit(10_000)
// 	buyOrderA := NewOrder(true, 5)
// 	buyOrderB := NewOrder(true, 10)
// 	buyOrderC := NewOrder(true, 15)
// 	l.AddOrder(buyOrderA)
// 	l.AddOrder(buyOrderB)
// 	l.AddOrder(buyOrderC)

// 	fmt.Println(l)

// 	l.DeleteOrder(buyOrderA)
// 	fmt.Println(l)

// 	l.DeleteOrder(buyOrderB)
// 	fmt.Println(l)

// 	l.DeleteOrder(buyOrderC)
// 	fmt.Println(l)

// 	buyOrder := NewOrder(true, 10)

// 	l.AddOrder(buyOrder)
// 	fmt.Println(l)
// }


// func TestPlaceLimitOrder(t *testing.T) {
// 	ordBook := NewOrderBook()

// 	sellOrderA := NewOrder(false, 10)
// 	sellOrderB := NewOrder(false, 5)
// 	ordBook.PlaceLimitOrder(sellOrderA,10_000)
// 	ordBook.PlaceLimitOrder(sellOrderB,8_000)

// 	assert(t,len(ordBook.asks),2)
// 	for i := 0; i < len(ordBook.asks); i++ {
// 		fmt.Println(ordBook.asks[i])
// 	}
// }

func TestPlaceMarketOrder(t *testing.T) {
	ordBook := NewOrderBook()

	// liquidity: two ask limits at different prices
	sellOrderA := NewOrder(false, 20)
	ordBook.PlaceLimitOrder(sellOrderA, 10_000)

	buyOrderA := NewOrder(true, 10)
	matches := ordBook.PlaceMarketOrder(buyOrderA)

	assert(t, len(matches), 1)
	assert(t, len(ordBook.asks), 1)
	assert(t, ordBook.AskTotalVolume(), 10.0)
	assert(t, matches[0].Price, 10_000.0)
	assert(t, matches[0].SizeFilled, 10.0)
	assert(t, buyOrderA.IsFilled(), true)

	for _, match := range matches {
		fmt.Printf("%+v",match)
	}
	
}

func TestPlaceMarketOrderWithMultipleMatches(t *testing.T) {
	ordBook := NewOrderBook()

	buyOrderA := NewOrder(true, 15)
	buyOrderB := NewOrder(true, 10)
	buyOrderC := NewOrder(true, 5)
	buyOrderD := NewOrder(true, 1)

	ordBook.PlaceLimitOrder(buyOrderC, 8_000)
	ordBook.PlaceLimitOrder(buyOrderB, 9_000)  
	ordBook.PlaceLimitOrder(buyOrderD, 9_000)
	ordBook.PlaceLimitOrder(buyOrderA, 10_000)

	assert(t, ordBook.BidTotalVolume(), 31.0)

	sellOrder := NewOrder(false, 27)
	matches := ordBook.PlaceMarketOrder(sellOrder)

	assert(t, len(matches), 4)
	assert(t, ordBook.BidTotalVolume(), 4.0)
	assert(t, len(ordBook.bids), 1)

	for _, match := range matches {
		fmt.Printf("%+v\n", match)
	}

}