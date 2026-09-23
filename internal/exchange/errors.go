package exchange

import "errors"

var (
	ErrUnknownSymbol         = errors.New("exchange: unknown symbol")
	ErrInvalidSize           = errors.New("exchange: size must be greater than zero")
	ErrInvalidPrice          = errors.New("exchange: price must be greater than zero")
	ErrInsufficientLiquidity = errors.New("exchange: insufficient liquidity to fill market order")
	ErrOrderNotFound         = errors.New("exchange: order not found")
)
