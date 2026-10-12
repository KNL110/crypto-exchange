package main

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

func EthClient(addr string) *ethclient.Client {
	client, err := ethclient.Dial("http://" + addr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Ethereum client")
	}
	log.Info().Msg("connected to Ethereum client")
	return client
}
