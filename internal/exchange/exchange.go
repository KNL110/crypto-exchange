package exchange

import (
	"fmt"
	"sync/atomic"

	matchengine "github.com/knl110/crypto-exchange/internal/MatchEngine"
)

// Exchange coordinates trading across all configured symbols.
type Exchange struct {
	orderBooks map[Symbol]*SymbolBook
	nextOrderID atomic.Uint64
}

func NewExchange() *Exchange {
	books := make(map[Symbol]*SymbolBook)
	return &Exchange{orderBooks: books}
}

func (e *Exchange) symbolBook(symbol Symbol) (*SymbolBook, error) {
	sb, ok := e.orderBooks[symbol]
	if !ok {
		return nil, ErrUnknownSymbol
	}
	return sb, nil
}

// Symbols returns the configured trading symbols.
func (e *Exchange) Symbols() []Symbol {
	symbols := make([]Symbol, 0, len(e.orderBooks))
	for s := range e.orderBooks {
		symbols = append(symbols, s)
	}
	return symbols
}

func (e *Exchange) placeLimitOrder(isBid bool,price float64,size uint64,sym Symbol) (*matchengine.Order,error){
	if size <= 0 {
		return nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w",ErrInvalidSize)
	}
	if price <= 0 {
		return nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w",ErrInvalidPrice)
	}

	sb, err := e.symbolBook(sym)
	if err != nil {
		return nil, fmt.Errorf("PlaceLimitOrder (Exchange): %w",err)
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()

	id := e.nextOrderID.Add(1)
	order := matchengine.NewOrder(id,isBid,int64(size))

	if err = sb.book.PlaceLimitOrder(order,price); err != nil {
		return nil,fmt.Errorf("PlaceLimitOrder (Exchange): %w",err)
	}
	return order,nil
}
