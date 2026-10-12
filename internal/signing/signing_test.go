package signing_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/knl110/crypto-exchange/internal/signing"
	"github.com/knl110/crypto-exchange/internal/signing/signingtest"
)

var chainID = big.NewInt(1337)

func TestRecoverSignerRoundTrip(t *testing.T) {
	alice := signingtest.NewTrader(t, chainID)

	order, sig := alice.Limit(true, 3_500, 10, "ETH")
	if got, err := signing.RecoverSigner(order.TypedData(chainID), sig); err != nil || got != alice.Addr {
		t.Fatalf("order signer = %s, %v; want %s", got, err, alice.Addr)
	}

	cancel, sig := alice.Cancel("ETH", 1)
	if got, err := signing.RecoverSigner(cancel.TypedData(chainID), sig); err != nil || got != alice.Addr {
		t.Fatalf("cancel signer = %s, %v; want %s", got, err, alice.Addr)
	}
}

func TestRecoverSignerAcceptsBothRecoveryIDStyles(t *testing.T) {
	alice := signingtest.NewTrader(t, chainID)
	order, sig := alice.Limit(true, 3_500, 10, "ETH") // wallet style: v = 27/28

	raw := append([]byte(nil), sig...)
	raw[crypto.RecoveryIDOffset] -= 27 // go-ethereum style: v = 0/1
	if got, err := signing.RecoverSigner(order.TypedData(chainID), raw); err != nil || got != alice.Addr {
		t.Fatalf("v=0/1 signer = %s, %v; want %s", got, err, alice.Addr)
	}
}

func TestRecoverSignerDetectsChanges(t *testing.T) {
	alice := signingtest.NewTrader(t, chainID)
	order, sig := alice.Limit(true, 3_500, 10, "ETH")

	changed := order
	changed.Size = 11
	if got, _ := signing.RecoverSigner(changed.TypedData(chainID), sig); got == alice.Addr {
		t.Fatalf("changed order still recovers to the maker")
	}
	if got, _ := signing.RecoverSigner(order.TypedData(big.NewInt(1)), sig); got == alice.Addr {
		t.Fatalf("signature for chain 1337 recovers to the maker on chain 1")
	}
	if _, err := signing.RecoverSigner(order.TypedData(chainID), sig[:10]); !errors.Is(err, signing.ErrInvalidSignature) {
		t.Fatalf("short signature: err = %v, want ErrInvalidSignature", err)
	}
}
