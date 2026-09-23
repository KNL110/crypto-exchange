package exchange

import (
	"sync"

	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
)

// SymbolBook pairs one symbol's order book with the mutex that serializes
// all access to it. Each symbol gets its own lock so unrelated symbols never
// contend with each other.
const(
	symETH = "ETH"
)


type SymbolBook struct {
	mu     sync.Mutex
	symbol string
	book   *matchengine.OrderBook
}

func newSymbolBook(symbol string) *SymbolBook {
	return &SymbolBook{
		symbol: symbol,
		book:   matchengine.NewOrderBook(),
	}
}
