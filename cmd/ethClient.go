package main

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
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
	address := common.HexToAddress("0xD460488273a9f643Cd075fe1b65a95f7c2F45257")
	balance, err := client.BalanceAt(ctx,address,nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get balance")
	}
	log.Info().Str("address", address.Hex()).Str("balance", balance.String()).Msg("balance retrieved")
	return client
}