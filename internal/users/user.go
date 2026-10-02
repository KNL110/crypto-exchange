package users

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
)

type User struct {
	ID         uint64
	privateKey *ecdsa.PrivateKey
}

func NewUser(privateKey string, id uint64) *User {
	privateKeyECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		panic(err)
	}
	return &User{
		ID:         id,
		privateKey: privateKeyECDSA,
	}
}
