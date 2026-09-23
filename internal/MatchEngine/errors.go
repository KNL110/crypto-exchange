package matchengine

import (
	"errors"
)

var ErrDuplicateOrderID = errors.New("matchengine: order id already in use")
var ErrOrderNotFound = errors.New("matchengine: order not found")
var ErrLimitNotFound = errors.New("matchengine: limit not found")
var ErrWrongLimit = errors.New("matchengine: order does not belong to this limit")
var ErrNilOrder = errors.New("matchengine: order is nil")
var ErrInsufficientLiquidity = errors.New("matchengine: order size exceeds available liquidity")
