package matchengine

import (
	"fmt"
	"testing"
)

func TestOrderBook(t *testing.T) {
	ordBook := NewOrderBook()

	buyOrderA := NewOrder(true, 10)
	buyOrderB := NewOrder(true, 2000)

	ordBook.PlaceOrder(buyOrderA, 10_000)
	ordBook.PlaceOrder(buyOrderB, 19_000)

	for i := 0; i < len(ordBook.Bids); i++ {
		fmt.Println(ordBook.Bids[i])
	}
}
