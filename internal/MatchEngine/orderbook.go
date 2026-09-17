package matchengine

import (
	"time" 
	"fmt"
)

//---------------------------------------------------------------------------------------------------

// Order represents a single buy or sell order resting at a price level.
type Order struct{
	Size float64
	Bid bool
	Limit *Limit
	Timestamp int64
}

func NewOrder(bid bool, size float64) *Order{
	return &Order{
		Size: size,
		Bid: bid,
		Timestamp: time.Now().UnixNano(),
	}
}

func (order *Order) String() string{
	return fmt.Sprintf("Order[size: %.2f]", order.Size)
}

//---------------------------------------------------------------------------------------------------

// Limit represents a single price level and the orders resting at it.
type Limit struct {
	Price float64
	Orders []*Order
	TotalVolume float64
}

func NewLimit(price float64) *Limit{
	return &Limit{
		Price: price,
		Orders: []*Order{},
	}
}

func (lim *Limit) AddOrder(order *Order){
	order.Limit = lim
	lim.Orders = append(lim.Orders, order)
	lim.TotalVolume += order.Size
}

func (lim *Limit) DeleteOrder(order *Order){
	ordLen := len(lim.Orders)

	for i, o := range lim.Orders{
		if o == order{
			lim.Orders[i] = lim.Orders[ordLen-1]
			lim.Orders[ordLen-1] = nil //avoid memory leak
			lim.Orders = lim.Orders[:ordLen-1]
			break
		}
	}
	order.Limit = nil    //remove the limit reference form order since it no longer in limit
	lim.TotalVolume -= order.Size

	//TODO: if lim.Orders is empty, remove the limit from the order book,sort to maintain FIFO order, etc.
}

//----------------------------------------------------------------------------------------------------

// OrderBook holds all price levels for both sides of the market.
type OrderBook struct{
	Asks []*Limit
	Bids []*Limit
}


