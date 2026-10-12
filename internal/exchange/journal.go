package exchange

import (
	"errors"
	"fmt"

	"github.com/knl110/crypto-exchange/internal/signing"
)

// ActionKind is what an Action does.
type ActionKind string

const (
	ActionPlaceLimit  ActionKind = "place_limit"
	ActionPlaceMarket ActionKind = "place_market"
	ActionCancel      ActionKind = "cancel"
)

// Action is one accepted order or cancel exactly as the maker signed it,
// plus the order ID and unix time (seconds) it was applied with. That is
// everything Replay needs to apply it again identically.
type Action struct {
	Kind      ActionKind
	Time      int64
	OrderID   uint64
	Order     signing.Order  // ActionPlaceLimit, ActionPlaceMarket
	Cancel    signing.Cancel // ActionCancel
	Signature []byte
}

// Journal durably records actions. Append must not return until the action
// is safely stored: the exchange applies it right after.
type Journal interface {
	Append(Action) error
}

// SetJournal makes the exchange record every accepted action to j before
// applying it. Set it after replaying the journal and before serving traffic.
func (e *Exchange) SetJournal(j Journal) {
	e.journal = j
}

// Replay applies an action read back from the journal, through the same
// checks and matching as a live request, at the action's original time and
// with its original order ID: the maker's signature is verified again,
// expiry is decided as it was then, and the nonce is reserved again. Replay
// actions in journal order, before serving traffic. An order that was
// rejected for liquidity when it was journaled is rejected the same way
// again, which is not an error.
func (e *Exchange) Replay(a Action) error {
	rp := &replayed{ts: a.Time, orderID: a.OrderID}
	switch a.Kind {
	case ActionPlaceLimit, ActionPlaceMarket:
		if a.OrderID > e.nextOrderID.Load() {
			e.nextOrderID.Store(a.OrderID)
		}
		_, err := e.placeOrder(a.Order, a.Signature, a.Kind == ActionPlaceMarket, rp)
		if err != nil && !errors.Is(err, ErrInsufficientLiquidity) {
			return err
		}
		return nil
	case ActionCancel:
		_, err := e.cancel(a.Cancel, a.Signature, rp)
		return err
	default:
		return fmt.Errorf("exchange: unknown action %q", a.Kind)
	}
}

// replayed carries what Replay takes from an Action instead of generating
// it live: the original time and order ID. nil means a live request.
type replayed struct {
	ts      int64
	orderID uint64
}

// opTime is the unix time (seconds) an operation runs at.
func (e *Exchange) opTime(rp *replayed) int64 {
	if rp != nil {
		return rp.ts
	}
	return e.now().Unix()
}

// logAction records a live action in the journal. Replayed actions are
// already in it.
func (e *Exchange) logAction(rp *replayed, a Action) error {
	if rp != nil || e.journal == nil {
		return nil
	}
	if err := e.journal.Append(a); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	return nil
}
