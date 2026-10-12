// Package signingtest provides a test wallet that signs orders and cancels
// the way a user's browser wallet would.
package signingtest

import (
	"crypto/ecdsa"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

// Trader is a wallet with its own key that signs for chainID. Each order it
// builds gets the next nonce and expires in an hour.
type Trader struct {
	Addr common.Address

	t       *testing.T
	key     *ecdsa.PrivateKey
	chainID *big.Int
	nonce   atomic.Uint64
}

// NewTrader returns a trader with a freshly generated key.
func NewTrader(t *testing.T, chainID *big.Int) *Trader {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return &Trader{Addr: crypto.PubkeyToAddress(key.PublicKey), t: t, key: key, chainID: chainID}
}

// Sign produces a wallet-style (v = 27/28) EIP-712 signature over td.
func (tr *Trader) Sign(td apitypes.TypedData) []byte {
	tr.t.Helper()
	hash, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		tr.t.Fatalf("TypedDataAndHash: %v", err)
	}
	sig, err := crypto.Sign(hash, tr.key)
	if err != nil {
		tr.t.Fatalf("Sign: %v", err)
	}
	sig[crypto.RecoveryIDOffset] += 27
	return sig
}

// NewOrder builds an unsigned limit order from this trader.
func (tr *Trader) NewOrder(isBid bool, size, price uint64, sym symbol.Symbol) signing.Order {
	return signing.Order{
		Maker:  tr.Addr,
		Symbol: sym,
		IsBid:  isBid,
		Size:   size,
		Price:  price,
		Nonce:  tr.nonce.Add(1),
		Expiry: uint64(time.Now().Add(time.Hour).Unix()),
	}
}

// Limit signs a limit order.
func (tr *Trader) Limit(isBid bool, price, size uint64, sym symbol.Symbol) (signing.Order, []byte) {
	msg := tr.NewOrder(isBid, size, price, sym)
	return msg, tr.Sign(msg.TypedData(tr.chainID))
}

// Market signs a market order that accepts fills up to worstPrice.
func (tr *Trader) Market(isBid bool, size, worstPrice uint64, sym symbol.Symbol) (signing.Order, []byte) {
	msg := tr.NewOrder(isBid, size, worstPrice, sym)
	msg.IsMarket = true
	return msg, tr.Sign(msg.TypedData(tr.chainID))
}

// Cancel signs a cancel of orderID.
func (tr *Trader) Cancel(sym symbol.Symbol, orderID uint64) (signing.Cancel, []byte) {
	msg := signing.Cancel{Maker: tr.Addr, Symbol: sym, OrderID: orderID}
	return msg, tr.Sign(msg.TypedData(tr.chainID))
}
