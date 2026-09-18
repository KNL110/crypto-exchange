package matchengine

import (
	"fmt"
	"sort"
	"time"
)


type Orders []*Order
type Limits []*Limit
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

func (ord Orders) Len() int {
	return len(ord)
}

func (ord Orders) Swap(i,j int){
	ord[i],ord[j] = ord[j],ord[i]
}

func (ord Orders) Less(i,j int) bool{
	return ord[i].Timestamp < ord[j].Timestamp
}

//---------------------------------------------------------------------------------------------------

// Limit represents a single price level and the orders resting at it.
type Limit struct {
	Price       float64
	Orders
	TotalVolume float64
}


func (lims Limits) Len() int {
	return len(lims)
}

func (lims Limits) Swap(i,j int){
	lims[i],lims[j] = lims[j],lims[i]
}

type BybestAsk struct{
	Limits
}

func (a BybestAsk) Less(i,j int) bool{
	return a.Limits[i].Price < a.Limits[j].Price
}

type BybestBid struct{
	Limits
}

func (b BybestBid) Less(i,j int) bool{
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

	//sort to maintain FIFO order
	sort.Sort(lim.Orders)
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
