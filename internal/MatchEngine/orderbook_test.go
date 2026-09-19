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

func TestPlaceLimitOrder(t *testing.T) {
	ordBook := NewOrderBook()

	sellOrderA := NewOrder(false, 10)
	sellOrderB := NewOrder(false, 5)
	ordBook.PlaceLimitOrder(sellOrderA,10_000)
	ordBook.PlaceLimitOrder(sellOrderB,8_000)

	assert(t,len(ordBook.asks),2)
	for i := 0; i < len(ordBook.bids); i++ {
		fmt.Println(ordBook.bids[i])
	}
}

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

	for _, match := range matches {
		fmt.Printf("%+v",match)
	}
	
}
