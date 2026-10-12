package exchange

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/rs/zerolog/log"

	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
	"github.com/knl110/crypto-exchange/internal/wal"
)

// Recover replays the write-ahead log at path to rebuild the order books and
// used nonces from a prior run, then opens that same file for all future
// writes. Call it exactly once, before the Exchange serves any traffic.
//
// Each record goes through the same code as a live request, at the record's
// original time and with its original order ID: the maker's signature is
// verified again, expiry is decided as it was live, and nonces are reserved
// again. Matching is deterministic, so the same records in the same order
// rebuild the same books. Because every record carries the maker's signature,
// a tampered log can't invent or alter orders without the makers' keys.
//
// hmacKey, if non-empty, additionally makes the log tamper-evident against
// deleting, reordering or duplicating records (see internal/wal/mac.go);
// any break in the chain makes Recover fail rather than start from a
// rewritten history.
func (e *Exchange) Recover(path string, hmacKey []byte) error {
	if len(hmacKey) == 0 {
		log.Warn().Msg("WAL_HMAC_KEY is not set: deleted or reordered write-ahead log records will not be detected")
	}

	var maxOrderID uint64
	lastSeq, lastMAC, truncated, err := wal.Replay(path, hmacKey, func(rec wal.Record) error {
		if rec.OrderID > maxOrderID {
			maxOrderID = rec.OrderID
		}
		return e.replay(rec)
	})
	if err != nil {
		return fmt.Errorf("exchange: recover: %w", err)
	}
	if truncated {
		log.Warn().Str("wal_path", path).Msg("discarded a truncated trailing wal record left by a prior crash")
	}
	if maxOrderID > e.nextOrderID.Load() {
		e.nextOrderID.Store(maxOrderID)
	}

	w, err := wal.Open(path, lastSeq+1, hmacKey, lastMAC)
	if err != nil {
		return fmt.Errorf("exchange: open wal for live writes: %w", err)
	}
	e.wal = w

	log.Info().Str("wal_path", path).Uint64("last_seq", lastSeq).Bool("integrity_checked", len(hmacKey) > 0).Msg("recovery complete")
	return nil
}

// replay applies one WAL record.
func (e *Exchange) replay(rec wal.Record) error {
	if !common.IsHexAddress(rec.Maker) {
		return fmt.Errorf("record seq=%d: invalid maker %q", rec.Seq, rec.Maker)
	}
	sig, err := hexutil.Decode(rec.Signature)
	if err != nil {
		return fmt.Errorf("record seq=%d: invalid signature: %w", rec.Seq, err)
	}
	maker := common.HexToAddress(rec.Maker)
	rp := &replayed{ts: rec.Timestamp, orderID: rec.OrderID}

	switch rec.Action {
	case wal.ActionPlaceLimit, wal.ActionPlaceMarket:
		market := rec.Action == wal.ActionPlaceMarket
		msg := signing.Order{
			Maker:    maker,
			Symbol:   symbol.Symbol(rec.Symbol),
			IsBid:    rec.IsBid,
			IsMarket: market,
			Size:     rec.Size,
			Price:    rec.Price,
			Nonce:    rec.Nonce,
			Expiry:   rec.Expiry,
		}
		// A market order that found too little liquidity was logged and then
		// rejected live; it is rejected the same way here.
		if _, err := e.placeOrder(msg, sig, market, rp); err != nil && !errors.Is(err, ErrInsufficientLiquidity) {
			return fmt.Errorf("record seq=%d: %w", rec.Seq, err)
		}

	case wal.ActionCancel:
		msg := signing.Cancel{Maker: maker, Symbol: symbol.Symbol(rec.Symbol), OrderID: rec.OrderID}
		if _, err := e.cancel(msg, sig, rp); err != nil {
			return fmt.Errorf("record seq=%d: %w", rec.Seq, err)
		}

	default:
		return fmt.Errorf("record seq=%d: unknown action %q", rec.Seq, rec.Action)
	}
	return nil
}

// Close closes the write-ahead log, if Recover opened one.
func (e *Exchange) Close() error {
	if e.wal == nil {
		return nil
	}
	return e.wal.Close()
}
