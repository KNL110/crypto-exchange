// Package symbol defines the markets the exchange trades and the per-symbol
// order book with the lock that serializes access to it.
package symbol

import (
	"sync"

	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
)

// Symbol identifies a market, e.g. "ETH".
type Symbol string

// Configured is every symbol the exchange trades. Each gets its own Book.
var Configured = []Symbol{"ETH", "BTC", "LTC"}

// Book pairs one symbol's order book with the mutex that serializes all
// access to it. Each symbol gets its own lock so unrelated symbols never
// contend with each other.
type Book struct {
	mu   sync.Mutex
	sym  Symbol
	book *matchengine.OrderBook

	// now is the unix time (seconds) of the operation currently holding mu.
	// The book's clock reads it, so an operation decides expiry at one fixed
	// time, which replay can reproduce exactly.
	now uint64
}

// NewBook returns an empty book for sym.
func NewBook(sym Symbol) *Book {
	b := &Book{sym: sym, book: matchengine.NewOrderBook()}
	b.book.SetClock(func() uint64 { return b.now })
	return b
}

// Symbol is the market this book is for.
func (b *Book) Symbol() Symbol {
	return b.sym
}

// Lock takes the book's lock for one operation running at unix time now
// (seconds) and returns the order book to work on. Every expiry decision
// inside the operation uses now. Call Unlock when done, and don't keep the
// order book after that.
func (b *Book) Lock(now uint64) *matchengine.OrderBook {
	b.mu.Lock()
	b.now = now
	return b.book
}

// Unlock releases the lock taken by Lock.
func (b *Book) Unlock() {
	b.mu.Unlock()
}
