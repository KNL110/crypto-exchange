package exchange

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/knl110/crypto-exchange/internal/symbol"
)

// Orders and cancels are EIP-712 typed data. A wallet signs them with
// eth_signTypedData_v4; the server recovers the signer and checks it is the
// maker. The server never holds user keys.
//
// TODO: add verifyingContract to the domain once the settlement contract is deployed.
const (
	domainName    = "CryptoExchange"
	domainVersion = "1"
)

var eip712Types = apitypes.Types{
	"EIP712Domain": {
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
	},
	"Order": {
		{Name: "maker", Type: "address"},
		{Name: "symbol", Type: "string"},
		{Name: "isBid", Type: "bool"},
		{Name: "isMarket", Type: "bool"},
		{Name: "size", Type: "uint256"},
		{Name: "price", Type: "uint256"},
		{Name: "nonce", Type: "uint256"},
		{Name: "expiry", Type: "uint256"},
	},
	"Cancel": {
		{Name: "maker", Type: "address"},
		{Name: "symbol", Type: "string"},
		{Name: "orderId", Type: "uint256"},
	},
}

// OrderMessage is the order a maker signs. Price is a whole number of ticks:
// a limit order's limit price, or for a market order the worst price the maker
// accepts. IsMarket is signed so one kind can't be submitted as the other.
// Expiry is a unix timestamp in seconds.
type OrderMessage struct {
	Maker    common.Address
	Symbol   symbol.Symbol
	IsBid    bool
	IsMarket bool
	Size     uint64
	Price    uint64
	Nonce    uint64
	Expiry   uint64
}

// CancelMessage is what a maker signs to cancel one of their resting orders.
type CancelMessage struct {
	Maker   common.Address
	Symbol  symbol.Symbol
	OrderID uint64
}

func domain(chainID *big.Int) apitypes.TypedDataDomain {
	return apitypes.TypedDataDomain{
		Name:    domainName,
		Version: domainVersion,
		ChainId: (*math.HexOrDecimal256)(chainID),
	}
}

func u256(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}

func (m OrderMessage) typedData(chainID *big.Int) apitypes.TypedData {
	return apitypes.TypedData{
		Types:       eip712Types,
		PrimaryType: "Order",
		Domain:      domain(chainID),
		Message: apitypes.TypedDataMessage{
			"maker":    m.Maker.Hex(),
			"symbol":   string(m.Symbol),
			"isBid":    m.IsBid,
			"isMarket": m.IsMarket,
			"size":     u256(m.Size),
			"price":    u256(m.Price),
			"nonce":    u256(m.Nonce),
			"expiry":   u256(m.Expiry),
		},
	}
}

func (m CancelMessage) typedData(chainID *big.Int) apitypes.TypedData {
	return apitypes.TypedData{
		Types:       eip712Types,
		PrimaryType: "Cancel",
		Domain:      domain(chainID),
		Message: apitypes.TypedDataMessage{
			"maker":   m.Maker.Hex(),
			"symbol":  string(m.Symbol),
			"orderId": u256(m.OrderID),
		},
	}
}

// recoverSigner returns the address that produced sig over the typed data.
// It accepts both v = 0/1 and the wallet-style v = 27/28.
func recoverSigner(td apitypes.TypedData, sig []byte) (common.Address, error) {
	if len(sig) != crypto.SignatureLength {
		return common.Address{}, ErrInvalidSignature
	}
	hash, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		return common.Address{}, err
	}
	s := make([]byte, len(sig))
	copy(s, sig)
	if s[crypto.RecoveryIDOffset] >= 27 {
		s[crypto.RecoveryIDOffset] -= 27
	}
	pub, err := crypto.SigToPub(hash, s)
	if err != nil {
		return common.Address{}, ErrInvalidSignature
	}
	return crypto.PubkeyToAddress(*pub), nil
}
