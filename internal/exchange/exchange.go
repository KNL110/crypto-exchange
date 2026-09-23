package exchange

import (
	"sync/atomic"
)

// Exchange coordinates trading across all configured symbols.
type Exchange struct {
	books map[Symbol]*SymbolBook
	nextOrderID atomic.Uint64
}

func NewExchange() *Exchange {
	books := make(map[Symbol]*SymbolBook)
	return &Exchange{books: books}
}

func (e *Exchange) symbolBook(symbol Symbol) (*SymbolBook, error) {
	sb, ok := e.books[symbol]
	if !ok {
		return nil, ErrUnknownSymbol
	}
	return sb, nil
}

// Symbols returns the configured trading symbols.
func (e *Exchange) Symbols() []Symbol {
	symbols := make([]Symbol, 0, len(e.books))
	for s := range e.books {
		symbols = append(symbols, s)
	}
	return symbols
}
