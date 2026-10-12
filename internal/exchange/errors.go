package exchange

import "errors"

var (
	ErrUnknownSymbol         = errors.New("exchange: unknown symbol")
	ErrInvalidSize           = errors.New("exchange: size must be greater than zero")
	ErrInvalidPrice          = errors.New("exchange: price must be greater than zero")
	ErrInsufficientLiquidity = errors.New("exchange: insufficient liquidity to fill market order")
	ErrOrderNotFound         = errors.New("exchange: order not found")
	ErrOrderTypeMismatch     = errors.New("exchange: signed order type (isMarket) does not match this endpoint")
	ErrInvalidSignature      = errors.New("exchange: signature does not match maker")
	ErrOrderExpired          = errors.New("exchange: order expired")
	ErrNonceUsed             = errors.New("exchange: nonce already used by this maker")
	ErrNotOrderOwner         = errors.New("exchange: order belongs to a different maker")
)
