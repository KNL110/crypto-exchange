// Package wal implements an append-only, JSON-lines write-ahead log of
// exchange trading actions, used to recover in-memory order book state
// after a crash or restart. It knows nothing about matching semantics: it
// only persists and replays records in the order they were appended.
package wal

// ActionType identifies what kind of trading action a Record represents.
type ActionType string

const (
	ActionPlaceLimit  ActionType = "place_limit"
	ActionPlaceMarket ActionType = "place_market"
	ActionCancel      ActionType = "cancel"
)

// Record is a single entry in the write-ahead log: one signed trading action
// exactly as the maker signed it, plus when it was applied. That is enough to
// replay it against an empty order book and deterministically reconstruct the
// same state, re-verifying the maker's signature along the way.
type Record struct {
	Seq       uint64     `json:"seq"`
	Timestamp int64      `json:"ts"` // unix seconds the action was applied at; replay runs at this time
	Symbol    string     `json:"symbol"`
	Action    ActionType `json:"action"`
	OrderID   uint64     `json:"order_id"`
	Maker     string     `json:"maker"`
	IsBid     bool       `json:"is_bid,omitempty"`
	Size      uint64     `json:"size,omitempty"`
	Price     uint64     `json:"price,omitempty"` // ticks: limit price, or a market order's worst price
	Nonce     uint64     `json:"nonce,omitempty"`
	Expiry    uint64     `json:"expiry,omitempty"`
	Signature string     `json:"sig"` // hex EIP-712 signature by Maker

	// MAC is a hex-encoded HMAC-SHA256 chaining this record to the one
	// before it (see mac.go), present only when a signing key is
	// configured. It makes the log tamper-evident: editing, inserting, or
	// reordering a record without knowing the key breaks the chain, which
	// Replay detects and refuses to proceed past. Empty when no key is
	// configured — the log is then exactly as trusting of its own contents
	// as a plain append-only file, i.e. not tamper-evident at all.
	MAC string `json:"mac,omitempty"`
}
