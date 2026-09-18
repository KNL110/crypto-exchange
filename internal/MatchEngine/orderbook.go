package matchengine

import (
	"fmt"
	"time"
)

//---------------------------------------------------------------------------------------------------

// Order represents a single buy or sell order resting at a price level.
type Order struct {
	Size      float64
	IsBid     bool
	Limit     *Limit
	Timestamp int64
}

func NewOrder(isbid bool, size float64) *Order {
	return &Order{
		Size:      size,
		IsBid:     isbid,
		Timestamp: time.Now().UnixNano(),
	}
}

func (order *Order) String() string {
	return fmt.Sprintf("Order[size: %.2f , isBid: %t]", order.Size, order.IsBid)
}

//---------------------------------------------------------------------------------------------------

// Limit represents a single price level and the orders resting at it.
type Limit struct {
	Price       float64
	Orders      []*Order
	TotalVolume float64
}

type Limits []*Limit

func (lims Limits) Len() int {
	return len(lims)
}

func (lims Limits) Swap(i,j int){
	lims[i],lims[j] = lims[j],lims[i]
}

type BybestAsk struct{
	Limits
}

func (a BybestAsk) Compare(i,j int) bool{
	return a.Limits[i].Price < a.Limits[j].Price
}

type BybestBid struct{
	Limits
}

func (b BybestBid) Compare(i,j int) bool{
	return b.Limits[i].Price > b.Limits[j].Price
}

func NewLimit(price float64) *Limit {
	return &Limit{
		Price:  price,
		Orders: []*Order{},
	}
}

func (lim *Limit) String() string {
	return fmt.Sprintf("Limit[Price: %.2f, TotalVolume: %.2f, Orders: %v]", lim.Price, lim.TotalVolume, lim.Orders)
}

func (lim *Limit) AddOrder(order *Order) {
	order.Limit = lim
	lim.Orders = append(lim.Orders, order)
	lim.TotalVolume += order.Size
}

func (lim *Limit) DeleteOrder(order *Order) {
	ordLen := len(lim.Orders)

	for i, o := range lim.Orders {
		if o == order {
			lim.Orders[i] = lim.Orders[ordLen-1]
			lim.Orders[ordLen-1] = nil //avoid memory leak
			lim.Orders = lim.Orders[:ordLen-1]
			break
		}
	}
	order.Limit = nil //remove the limit reference form order since it no longer in limit
	lim.TotalVolume -= order.Size

	//TODO: if lim.Orders is empty, remove the limit from the order book,sort to maintain FIFO order, etc.
}

//----------------------------------------------------------------------------------------------------

type MatchedOrder struct {
	Ask        *Order
	Bid        *Order
	SizeFilled float64
	Price      float64
}

//----------------------------------------------------------------------------------------------------

// OrderBook holds all price levels for both sides of the market.
type OrderBook struct {
	Asks      []*Limit
	Bids      []*Limit
	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Asks:      []*Limit{},
		Bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.Asks, ordBook.Bids)
}

// match the order
// if order is not fully filled, add it to the order book
func (ordBook *OrderBook) PlaceOrder(order *Order, price float64) []MatchedOrder {
	//matching logic:

	if order.Size > 0 {
		ordBook.addOrder(order, price)
	}

	return []MatchedOrder{}
}

// add order to limit and add limit to order book if it does not exist
// depending on the order type, add to bid or ask limits
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

//----------------------------------------------------------------------------------------------------
