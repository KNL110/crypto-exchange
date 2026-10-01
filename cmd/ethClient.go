package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)



func EthClient(addr string) *ethclient.Client {
	client, err := ethclient.Dial("http://" + addr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Ethereum client")
	}
	log.Info().Msg("connected to Ethereum client")

	ctx := context.Background()
	address := common.HexToAddress("0xdA6C04444Ed8921e19Ea836Bce0Dad687a33725f")
	balance, err := client.BalanceAt(ctx,address,nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get balance")
	}
	log.Info().Str("address", address.Hex()).Str("balance", balance.String()).Msg("balance retrieved")
	return client
}

func transferEth(client *ethclient.Client, privateKey string) {
	pKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse private key")
	}

	publicKey := pKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal().Msg("failed to assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get nonce")
	}

	value := big.NewInt(1000000000000000000) // in wei (1 eth)
    gasLimit := uint64(21000)                // in units
    gasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        log.Fatal().Err(err).Msg("failed to get gas price")
    }
	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
    var data []byte
    tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

    chainID, err := client.NetworkID(context.Background())
    if err != nil {
        log.Fatal().Err(err).Msg("failed to get network ID")
    }
	chainID = big.NewInt(1337) // Set the chain ID to 1337 for Ganache
    signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), pKey)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to sign transaction")
    }

    err = client.SendTransaction(context.Background(), signedTx)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to send transaction")
    }

    fmt.Printf("tx sent: %s", signedTx.Hash().Hex())
}