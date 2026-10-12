package matchengine

import (
	"fmt"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// OrderBook holds all price levels for both sides of the market.
// Prices are whole numbers of ticks.
type OrderBook struct {
	asks      []*Limit
	bids      []*Limit
	AskLimits map[uint64]*Limit
	BidLimits map[uint64]*Limit
	orders    map[uint64]*Order

	// now returns the current unix time in seconds; tests replace it.
	now func() uint64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[uint64]*Limit),
		BidLimits: make(map[uint64]*Limit),
		orders:    make(map[uint64]*Order),
		now:       func() uint64 { return uint64(time.Now().Unix()) },
	}
}

func (ordBook *OrderBook) String() string {
	return fmt.Sprintf("OrderBook[Asks: %v, Bids: %v]", ordBook.Asks(), ordBook.Bids())
}

func (ordBook *OrderBook) Asks() []*Limit {
	sort.Sort(BybestAsk{ordBook.asks})
	return ordBook.asks
}

func (ordBook *OrderBook) Bids() Limits {
	sort.Sort(BybestBid{ordBook.bids})
	return ordBook.bids
}

func (ordBook *OrderBook) GetOrder(id uint64) (*Order, bool) {
	order, ok := ordBook.orders[id]
	return order, ok
}

// PlaceLimitOrder first trades the order against the opposite side at every
// price level its limit allows (a buy against asks <= price, a sell against
// bids >= price), each fill at the resting order's price. Whatever is left
// then rests in the book at price. The returned matches are empty if the
// order didn't cross.
func (ordBook *OrderBook) PlaceLimitOrder(order *Order, price uint64) ([]MatchedOrder, error) {
	if order.ID != 0 {
		if _, exists := ordBook.orders[order.ID]; exists {
			return nil, fmt.Errorf("PlaceLimitOrder -> %w", ErrDuplicateOrderID)
		}
	}

	ordBook.purgeExpired(!order.IsBid)
	matches := ordBook.match(order, price)
	if order.IsFilled() {
		return matches, nil
	}

	if order.IsBid {
		lim, exists := ordBook.BidLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.BidLimits[price] = lim
			ordBook.bids = append(ordBook.bids, lim)
		}
		lim.AddOrder(order)
	} else {
		lim, exists := ordBook.AskLimits[price]
		if !exists {
			lim = NewLimit(price)
			ordBook.AskLimits[price] = lim
			ordBook.asks = append(ordBook.asks, lim)
		}
		lim.AddOrder(order)
	}
	if order.ID != 0 {
		ordBook.orders[order.ID] = order
	}
	return matches, nil
}

// PlaceMarketOrder fills the whole order immediately, best price first, but
// never at a price worse than worstPrice (above it for a buy, below it for a
// sell). If the book can't fill all of it within that bound, nothing is
// filled and ErrInsufficientLiquidity is returned.
func (ordBook *OrderBook) PlaceMarketOrder(order *Order, worstPrice uint64) ([]MatchedOrder, error) {
	ordBook.purgeExpired(!order.IsBid)
	if order.Size > ordBook.crossableVolume(order.IsBid, worstPrice, order.Maker) {
		return nil, fmt.Errorf("PlaceMarketOrder -> %w", ErrInsufficientLiquidity)
	}
	return ordBook.match(order, worstPrice), nil
}

// crosses reports whether a resting level at levelPrice can trade with an
// incoming order on side isBid limited to price.
func crosses(isBid bool, levelPrice, price uint64) bool {
	if isBid {
		return levelPrice <= price
	}
	return levelPrice >= price
}

// opposite returns the side an incoming order on side isBid trades against,
// sorted best price first.
func (ordBook *OrderBook) opposite(isBid bool) []*Limit {
	if isBid {
		return ordBook.Asks()
	}
	return ordBook.Bids()
}

// crossableVolume is the resting volume an incoming order from maker could
// trade against without going past price. The maker's own resting orders
// don't count: self-trade prevention cancels them instead of filling.
func (ordBook *OrderBook) crossableVolume(isBid bool, price uint64, maker common.Address) uint64 {
	var total uint64
	for _, lim := range ordBook.opposite(isBid) {
		if !crosses(isBid, lim.Price, price) {
			break
		}
		total += lim.TotalVolume
		for o := lim.head; o != nil; o = o.next {
			if o.Maker == maker {
				total -= o.Size
			}
		}
	}
	return total
}

// match trades order against the opposite side, best price first, until it
// is filled or the next level is past price. Filled resting orders, the
// maker's own crossing orders (self-trade prevention) and emptied levels are
// removed from the book.
func (ordBook *OrderBook) match(order *Order, price uint64) []MatchedOrder {
	matches := []MatchedOrder{}
	cancelled := []*Order{}
	toDelete := []*Limit{}

	for _, lim := range ordBook.opposite(order.IsBid) {
		if order.IsFilled() || !crosses(order.IsBid, lim.Price, price) {
			break
		}
		lim.FillOrder(order, &matches, &cancelled)
		if lim.IsEmpty() {
			toDelete = append(toDelete, lim)
		}
	}

	for _, o := range cancelled {
		delete(ordBook.orders, o.ID)
	}

	for _, m := range matches {
		resting := m.Bid
		if order.IsBid {
			resting = m.Ask
		}
		if resting.IsFilled() {
			delete(ordBook.orders, resting.ID)
		}
	}
	if len(toDelete) > 0 {
		for _, lim := range toDelete {
			ordBook.removeLimit(lim, !order.IsBid)
		}
		ordBook.DeleteLimit(nil, !order.IsBid)
	}
	return matches
}

// purgeExpired removes every expired order from one side of the book, so
// matching never trades against an order its maker no longer stands behind.
// TODO: this walks the whole side on every incoming order; index orders by
// expiry if the book gets large.
func (ordBook *OrderBook) purgeExpired(isBid bool) {
	side := ordBook.asks
	if isBid {
		side = ordBook.bids
	}
	now := ordBook.now()

	emptied := []*Limit{}
	for _, lim := range side {
		for o := lim.head; o != nil; {
			next := o.next
			if o.isExpired(now) {
				lim.DeleteOrder(o)
				delete(ordBook.orders, o.ID)
			}
			o = next
		}
		if lim.IsEmpty() {
			emptied = append(emptied, lim)
		}
	}
	for _, lim := range emptied {
		ordBook.removeLimit(lim, isBid)
	}
}

func (ordBook *OrderBook) removeLimit(lim *Limit, isBid bool) {
	if isBid {
		delete(ordBook.BidLimits, lim.Price)
		for i, l := range ordBook.bids {
			if l == lim {
				ordBook.bids[i], ordBook.bids[len(ordBook.bids)-1] = ordBook.bids[len(ordBook.bids)-1], nil
				ordBook.bids = ordBook.bids[:len(ordBook.bids)-1]
				break
			}
		}
	} else {
		delete(ordBook.AskLimits, lim.Price)
		for i, l := range ordBook.asks {
			if l == lim {
				ordBook.asks[i], ordBook.asks[len(ordBook.asks)-1] = ordBook.asks[len(ordBook.asks)-1], nil
				ordBook.asks = ordBook.asks[:len(ordBook.asks)-1]
				break
			}
		}
	}
}

func (ordBook *OrderBook) DeleteLimit(lim *Limit, isBid bool) {
	if lim != nil {
		ordBook.removeLimit(lim, isBid)
	}
	if isBid {
		sort.Sort(BybestBid{ordBook.bids})
	} else {
		sort.Sort(BybestAsk{ordBook.asks})
	}
}

func (ordBook *OrderBook) BidTotalVolume() uint64 {
	var total uint64 = 0
	for _, lim := range ordBook.bids {
		total += lim.TotalVolume
	}
	return total
}

func (ordBook *OrderBook) AskTotalVolume() uint64 {
	var total uint64 = 0
	for _, lim := range ordBook.asks {
		total += lim.TotalVolume
	}
	return total
}

func (ordBook *OrderBook) CancelOrder(id uint64) (*Order, error) {
	order, ok := ordBook.orders[id]
	if !ok {
		return nil, fmt.Errorf("CancelOrder -> %w", ErrOrderNotFound)
	}

	lim := order.Limit
	if err := lim.DeleteOrder(order); err != nil {
		return nil, fmt.Errorf("CancelOrder -> %w", err)
	}
	delete(ordBook.orders, id)

	if lim.IsEmpty() {
		ordBook.DeleteLimit(lim, order.IsBid)
	}
	return order, nil
}
