package matchengine

import (
	"fmt"
	"testing"
)

func TestLimit(t *testing.T){
	l := NewLimit(10_000)
	buyOrderA := NewOrder(true,5)
	buyOrderB := NewOrder(true,10)
	buyOrderC := NewOrder(true,15)
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

	buyOrder := NewOrder(true, 10)

	l.AddOrder(buyOrder)
	fmt.Println(l)
}

func TestOrderBook(t *testing.T){
	ordBook := NewOrderBook()
	
	buyOrderA := NewOrder(true, 10)
	buyOrderB := NewOrder(true, 2000)

	ordBook.PlaceOrder(buyOrderA, 10_000)
	ordBook.PlaceOrder(buyOrderB, 10_000)

	for i:=0; i < len(ordBook.Bids); i++ {
		fmt.Println(ordBook.Bids[i])
	}

}