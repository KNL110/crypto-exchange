// Package recovery persists the exchange's actions to the write-ahead log
// and rebuilds the exchange from it on startup.
package recovery

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/rs/zerolog/log"

	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
	"github.com/knl110/crypto-exchange/internal/wal"
)

// Recover replays the write-ahead log at path into ex to rebuild its order
// books and used nonces from a prior run, then opens that same file and
// makes ex journal every future action to it. Call it once, before ex serves
// any traffic, and Close the returned log on shutdown.
//
// Replay goes through the exchange's normal checks (see exchange.Replay), so
// every record's signature is verified again: a tampered log can't invent or
// alter orders without the makers' keys.
//
// hmacKey, if non-empty, additionally makes the log tamper-evident against
// deleting, reordering or duplicating records (see internal/wal/mac.go);
// any break in the chain makes Recover fail rather than start from a
// rewritten history.
func Recover(ex *exchange.Exchange, path string, hmacKey []byte) (*wal.WAL, error) {
	if len(hmacKey) == 0 {
		log.Warn().Msg("WAL_HMAC_KEY is not set: deleted or reordered write-ahead log records will not be detected")
	}

	lastSeq, lastMAC, truncated, err := wal.Replay(path, hmacKey, func(rec wal.Record) error {
		a, err := fromRecord(rec)
		if err != nil {
			return err
		}
		if err := ex.Replay(a); err != nil {
			return fmt.Errorf("record seq=%d: %w", rec.Seq, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("recovery: %w", err)
	}
	if truncated {
		log.Warn().Str("wal_path", path).Msg("discarded a truncated trailing wal record left by a prior crash")
	}

	w, err := wal.Open(path, lastSeq+1, hmacKey, lastMAC)
	if err != nil {
		return nil, fmt.Errorf("recovery: open wal for live writes: %w", err)
	}
	ex.SetJournal(journal{w})

	log.Info().Str("wal_path", path).Uint64("last_seq", lastSeq).Bool("integrity_checked", len(hmacKey) > 0).Msg("recovery complete")
	return w, nil
}

// journal is an exchange.Journal backed by the write-ahead log.
type journal struct {
	w *wal.WAL
}

func (j journal) Append(a exchange.Action) error {
	_, err := j.w.Append(toRecord(a))
	return err
}

func toRecord(a exchange.Action) wal.Record {
	rec := wal.Record{
		Timestamp: a.Time,
		Action:    wal.ActionType(a.Kind),
		OrderID:   a.OrderID,
		Signature: hexutil.Encode(a.Signature),
	}
	if a.Kind == exchange.ActionCancel {
		rec.Symbol = string(a.Cancel.Symbol)
		rec.Maker = a.Cancel.Maker.Hex()
		return rec
	}
	o := a.Order
	rec.Symbol = string(o.Symbol)
	rec.Maker = o.Maker.Hex()
	rec.IsBid = o.IsBid
	rec.Size = o.Size
	rec.Price = o.Price
	rec.Nonce = o.Nonce
	rec.Expiry = o.Expiry
	return rec
}

func fromRecord(rec wal.Record) (exchange.Action, error) {
	if !common.IsHexAddress(rec.Maker) {
		return exchange.Action{}, fmt.Errorf("record seq=%d: invalid maker %q", rec.Seq, rec.Maker)
	}
	sig, err := hexutil.Decode(rec.Signature)
	if err != nil {
		return exchange.Action{}, fmt.Errorf("record seq=%d: invalid signature: %w", rec.Seq, err)
	}
	maker := common.HexToAddress(rec.Maker)
	sym := symbol.Symbol(rec.Symbol)

	a := exchange.Action{
		Kind:      exchange.ActionKind(rec.Action),
		Time:      rec.Timestamp,
		OrderID:   rec.OrderID,
		Signature: sig,
	}
	switch a.Kind {
	case exchange.ActionPlaceLimit, exchange.ActionPlaceMarket:
		a.Order = signing.Order{
			Maker:    maker,
			Symbol:   sym,
			IsBid:    rec.IsBid,
			IsMarket: a.Kind == exchange.ActionPlaceMarket,
			Size:     rec.Size,
			Price:    rec.Price,
			Nonce:    rec.Nonce,
			Expiry:   rec.Expiry,
		}
	case exchange.ActionCancel:
		a.Cancel = signing.Cancel{Maker: maker, Symbol: sym, OrderID: rec.OrderID}
	default:
		return exchange.Action{}, fmt.Errorf("record seq=%d: unknown action %q", rec.Seq, rec.Action)
	}
	return a, nil
}
