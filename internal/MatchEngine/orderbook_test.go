package matchengine

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	buyer  = common.HexToAddress("0x00000000000000000000000000000000000000b1")
	seller = common.HexToAddress("0x00000000000000000000000000000000000000a1")
)

func assert(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v != %+v", a, b)
	}
}

func TestLimit(t *testing.T) {
	l := NewLimit(10_000)
	buyOrderA := NewOrder(1, true, 5, buyer)
	buyOrderB := NewOrder(2, true, 10, buyer)
	buyOrderC := NewOrder(3, true, 15, buyer)
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

	buyOrder := NewOrder(4, true, 10, buyer)

	l.AddOrder(buyOrder)
	fmt.Println(l)
}

func TestPlaceLimitOrder(t *testing.T) {
	ordBook := NewOrderBook()

	sellOrderA := NewOrder(5, false, 10, seller)
	sellOrderB := NewOrder(6, false, 5, seller)
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
	sellOrderA := NewOrder(7, false, 20, seller)
	ordBook.PlaceLimitOrder(sellOrderA, 10_000)

	buyOrderA := NewOrder(8, true, 10, buyer)
	matches, _, err := ordBook.PlaceMarketOrder(buyOrderA, 10_000)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}

	assert(t, len(matches), 1)
	assert(t, len(ordBook.asks), 1)
	assert(t, ordBook.AskTotalVolume(), uint64(10))
	assert(t, matches[0].Price, uint64(10_000))
	assert(t, matches[0].SizeFilled, uint64(10))
	assert(t, buyOrderA.IsFilled(), true)

	for _, match := range matches {
		fmt.Printf("%+v", match)
	}

}

func TestPlaceMarketOrderWithMultipleMatches(t *testing.T) {
	ordBook := NewOrderBook()

	buyOrderA := NewOrder(9, true, 15, buyer)
	buyOrderB := NewOrder(10, true, 10, buyer)
	buyOrderC := NewOrder(11, true, 5, buyer)
	buyOrderD := NewOrder(12, true, 1, buyer)

	ordBook.PlaceLimitOrder(buyOrderC, 8_000)
	ordBook.PlaceLimitOrder(buyOrderB, 9_000)
	ordBook.PlaceLimitOrder(buyOrderD, 9_000)
	ordBook.PlaceLimitOrder(buyOrderA, 10_000)

	assert(t, ordBook.BidTotalVolume(), uint64(31))

	sellOrder := NewOrder(13, false, 27, seller)
	matches, _, err := ordBook.PlaceMarketOrder(sellOrder, 1)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}

	assert(t, len(matches), 4)
	assert(t, ordBook.BidTotalVolume(), uint64(4))
	assert(t, len(ordBook.bids), 1)

	for _, match := range matches {
		fmt.Printf("%+v\n", match)
	}

}

func TestOrderLookupBy(t *testing.T) {
	ordBook := NewOrderBook()

	order := NewOrder(42, true, 10, buyer)
	ordBook.PlaceLimitOrder(order, 9_000)

	got, ok := ordBook.GetOrder(42)
	assert(t, ok, true)
	assert(t, got, order)

	_, ok = ordBook.GetOrder(999)
	assert(t, ok, false)
}

func TestCancelOrder(t *testing.T) {
	ordBook := NewOrderBook()

	orderA := NewOrder(1, true, 10, buyer)
	orderB := NewOrder(2, true, 5, buyer)
	orderc := NewOrder(2, true, 10, buyer) // duplicate ID to test error handling
	ordBook.PlaceLimitOrder(orderA, 9_000)
	ordBook.PlaceLimitOrder(orderB, 9_000)
	_, _, err := ordBook.PlaceLimitOrder(orderc, 9_000)
	if err == nil || !errors.Is(err, ErrDuplicateOrderID) {
		t.Fatalf("PlaceLimitOrder: err = %v want err dup", err)
	}
	assert(t, ordBook.BidTotalVolume(), uint64(15))

	cancelled, err := ordBook.CancelOrder(orderA.ID)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	assert(t, cancelled, orderA)
	assert(t, ordBook.BidTotalVolume(), uint64(5))
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

	orderA := NewOrder(7, true, 10, buyer)
	if _, _, err := ordBook.PlaceLimitOrder(orderA, 9_000); err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}

	orderB := NewOrder(7, true, 5, buyer)
	_, _, err := ordBook.PlaceLimitOrder(orderB, 9_500)
	if !errors.Is(err, ErrDuplicateOrderID) {
		t.Fatalf("PlaceLimitOrder (duplicate id): err = %v, want ErrDuplicateOrderID", err)
	}

	// The book must be unchanged: the duplicate was rejected before any
	// mutation, not partially applied.
	assert(t, ordBook.BidTotalVolume(), uint64(10))
	assert(t, len(ordBook.bids), 1)
}
func TestPlaceMarketOrderInsufficientLiquidity(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 5, seller), 10_000)

	_, _, err := ordBook.PlaceMarketOrder(NewOrder(2, true, 10, buyer), 10_000)
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}
	assert(t, ordBook.AskTotalVolume(), uint64(5))
}

func TestLimitOrderCrossesBook(t *testing.T) {
	ordBook := NewOrderBook()
	sell := NewOrder(1, false, 10, seller)
	ordBook.PlaceLimitOrder(sell, 3_500)

	// A buy at 3600 crosses the 3500 ask: it trades at the resting price.
	buy := NewOrder(2, true, 10, buyer)
	matches, _, err := ordBook.PlaceLimitOrder(buy, 3_600)
	if err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	assert(t, len(matches), 1)
	assert(t, matches[0].Price, uint64(3_500))
	assert(t, matches[0].SizeFilled, uint64(10))
	assert(t, buy.IsFilled(), true)
	assert(t, sell.IsFilled(), true)

	// Nothing rests and the book is not crossed.
	assert(t, ordBook.AskTotalVolume(), uint64(0))
	assert(t, ordBook.BidTotalVolume(), uint64(0))
	assert(t, len(ordBook.asks), 0)
	assert(t, len(ordBook.bids), 0)
	_, ok := ordBook.GetOrder(sell.ID)
	assert(t, ok, false)
	_, ok = ordBook.GetOrder(buy.ID)
	assert(t, ok, false)
}

func TestLimitOrderPartialCrossRestsRemainder(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 5, seller), 3_400)
	ordBook.PlaceLimitOrder(NewOrder(2, false, 5, seller), 3_500)
	ordBook.PlaceLimitOrder(NewOrder(3, false, 5, seller), 3_700) // above the buy's limit

	buy := NewOrder(4, true, 20, buyer)
	matches, _, err := ordBook.PlaceLimitOrder(buy, 3_600)
	if err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}

	// Fills 3400 then 3500, stops before 3700.
	assert(t, len(matches), 2)
	assert(t, matches[0].Price, uint64(3_400))
	assert(t, matches[1].Price, uint64(3_500))
	assert(t, buy.Filled, uint64(10))

	// The unfilled 10 rests at the buy's own limit price.
	assert(t, buy.Size, uint64(10))
	assert(t, buy.Limit.Price, uint64(3_600))
	assert(t, ordBook.BidTotalVolume(), uint64(10))
	assert(t, ordBook.AskTotalVolume(), uint64(5))
	got, ok := ordBook.GetOrder(buy.ID)
	assert(t, ok, true)
	assert(t, got, buy)
}

func TestLimitOrderNotCrossingRests(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 10, seller), 3_500)

	matches, _, err := ordBook.PlaceLimitOrder(NewOrder(2, true, 10, buyer), 3_400)
	if err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}
	assert(t, len(matches), 0)
	assert(t, ordBook.AskTotalVolume(), uint64(10))
	assert(t, ordBook.BidTotalVolume(), uint64(10))
}

func TestMarketOrderRespectsWorstPrice(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 5, seller), 3_500)
	ordBook.PlaceLimitOrder(NewOrder(2, false, 5, seller), 9_000)

	// 10 are on the book, but only 5 are at or below 4000: rejected, nothing filled.
	_, _, err := ordBook.PlaceMarketOrder(NewOrder(3, true, 10, buyer), 4_000)
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}
	assert(t, ordBook.AskTotalVolume(), uint64(10))

	// Within the bound it fills.
	buy := NewOrder(4, true, 5, buyer)
	matches, _, err := ordBook.PlaceMarketOrder(buy, 4_000)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}
	assert(t, len(matches), 1)
	assert(t, matches[0].Price, uint64(3_500))
	assert(t, buy.Filled, uint64(5))
}

func TestExpiredOrdersAreNotMatched(t *testing.T) {
	ordBook := NewOrderBook()
	now := uint64(1_000)
	ordBook.SetClock(func() uint64 { return now })

	stale := NewOrder(1, false, 10, seller)
	stale.Expiry = 1_500
	ordBook.PlaceLimitOrder(stale, 3_400)
	fresh := NewOrder(2, false, 10, seller)
	ordBook.PlaceLimitOrder(fresh, 3_500) // Expiry 0: never expires

	now = 2_000 // stale has expired

	buy := NewOrder(3, true, 10, buyer)
	matches, _, err := ordBook.PlaceMarketOrder(buy, 10_000)
	if err != nil {
		t.Fatalf("PlaceMarketOrder: %v", err)
	}
	// Skips the cheaper expired order and fills against the live one.
	assert(t, len(matches), 1)
	assert(t, matches[0].Ask, fresh)
	assert(t, matches[0].Price, uint64(3_500))

	// The expired order is gone from the book.
	_, ok := ordBook.GetOrder(stale.ID)
	assert(t, ok, false)
	assert(t, len(ordBook.asks), 0)
}

func TestExpiredOrdersDontCountAsLiquidity(t *testing.T) {
	ordBook := NewOrderBook()
	now := uint64(1_000)
	ordBook.SetClock(func() uint64 { return now })

	stale := NewOrder(1, true, 10, buyer)
	stale.Expiry = 1_500
	ordBook.PlaceLimitOrder(stale, 3_500)
	now = 2_000

	_, _, err := ordBook.PlaceMarketOrder(NewOrder(2, false, 10, seller), 1)
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}
	assert(t, ordBook.BidTotalVolume(), uint64(0))
}

func TestFilledTracksPartialFills(t *testing.T) {
	ordBook := NewOrderBook()
	sell := NewOrder(1, false, 10, seller)
	ordBook.PlaceLimitOrder(sell, 3_500)

	ordBook.PlaceMarketOrder(NewOrder(2, true, 3, buyer), 3_500)
	ordBook.PlaceMarketOrder(NewOrder(3, true, 4, buyer), 3_500)

	assert(t, sell.Filled, uint64(7))
	assert(t, sell.Size, uint64(3))
}

func TestSelfTradeCancelsRestingOrder(t *testing.T) {
	ordBook := NewOrderBook()
	own := NewOrder(1, false, 10, buyer) // buyer's own resting sell
	ordBook.PlaceLimitOrder(own, 3_400)
	other := NewOrder(2, false, 10, seller)
	ordBook.PlaceLimitOrder(other, 3_500)

	buy := NewOrder(3, true, 10, buyer)
	matches, cancelled, err := ordBook.PlaceLimitOrder(buy, 3_600)
	if err != nil {
		t.Fatalf("PlaceLimitOrder: %v", err)
	}

	// Never trades with itself: the own 3400 sell is cancelled (and reported), the fill is against the seller.
	assert(t, cancelled, []*Order{own})
	assert(t, len(matches), 1)
	assert(t, matches[0].Ask, other)
	assert(t, own.Filled, uint64(0))
	_, ok := ordBook.GetOrder(own.ID)
	assert(t, ok, false)
	assert(t, ordBook.AskTotalVolume(), uint64(0))
	assert(t, len(ordBook.asks), 0)
}

func TestSelfTradeDoesNotCountAsLiquidity(t *testing.T) {
	ordBook := NewOrderBook()
	own := NewOrder(1, false, 10, buyer)
	ordBook.PlaceLimitOrder(own, 3_500)

	// The only liquidity is the buyer's own: rejected, and the book is untouched.
	_, _, err := ordBook.PlaceMarketOrder(NewOrder(2, true, 10, buyer), 10_000)
	if !errors.Is(err, ErrInsufficientLiquidity) {
		t.Fatalf("PlaceMarketOrder: err = %v, want ErrInsufficientLiquidity", err)
	}
	_, ok := ordBook.GetOrder(own.ID)
	assert(t, ok, true)
	assert(t, ordBook.AskTotalVolume(), uint64(10))
}

func TestSelfTradeLimitOrderRestsWithoutCrossing(t *testing.T) {
	ordBook := NewOrderBook()
	ordBook.PlaceLimitOrder(NewOrder(1, false, 10, buyer), 3_500)

	// Buying above your own ask cancels the ask; the buy rests, the book isn't crossed.
	buy := NewOrder(2, true, 10, buyer)
	matches, _, _ := ordBook.PlaceLimitOrder(buy, 3_600)
	assert(t, len(matches), 0)
	assert(t, ordBook.AskTotalVolume(), uint64(0))
	assert(t, ordBook.BidTotalVolume(), uint64(10))
}

func TestPurgeExpiredBothSides(t *testing.T) {
	ordBook := NewOrderBook()
	now := uint64(1_000)
	ordBook.SetClock(func() uint64 { return now })

	staleBid := NewOrder(1, true, 10, buyer)
	staleBid.Expiry = 1_500
	ordBook.PlaceLimitOrder(staleBid, 3_000)
	staleAsk := NewOrder(2, false, 10, seller)
	staleAsk.Expiry = 1_500
	ordBook.PlaceLimitOrder(staleAsk, 4_000)
	live := NewOrder(3, true, 5, buyer)
	ordBook.PlaceLimitOrder(live, 2_900)

	now = 2_000
	ordBook.PurgeExpired()

	assert(t, ordBook.BidTotalVolume(), uint64(5))
	assert(t, ordBook.AskTotalVolume(), uint64(0))
	assert(t, len(ordBook.bids), 1)
	assert(t, len(ordBook.asks), 0)
	_, ok := ordBook.GetOrder(staleBid.ID)
	assert(t, ok, false)
	_, ok = ordBook.GetOrder(staleAsk.ID)
	assert(t, ok, false)
	_, ok = ordBook.GetOrder(live.ID)
	assert(t, ok, true)
}
