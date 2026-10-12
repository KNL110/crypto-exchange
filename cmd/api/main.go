package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/knl110/crypto-exchange/internal/api"
	"github.com/knl110/crypto-exchange/internal/config"
	"github.com/knl110/crypto-exchange/internal/exchange"
	"github.com/knl110/crypto-exchange/internal/logging"
	"github.com/knl110/crypto-exchange/internal/recovery"
)

func main() {
	cfg := config.MustLoad()
	log.Logger = logging.New(cfg.LogLevel, cfg.LogPretty)

	ethClient := EthClient(cfg.GanacheServer.GetAddr())
	defer ethClient.Close()
	chainID, err := ethClient.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}

	ex, err := exchange.NewExchange(cfg.GanacheServer.PrivateKey, chainID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create exchange")
	}
	walLog, err := recovery.Recover(ex, cfg.WALPath, []byte(cfg.WALHMACKey))
	if err != nil {
		log.Fatal().Err(err).Str("wal_path", cfg.WALPath).Msg("failed to recover exchange state from wal")
	}
	defer func() {
		if err := walLog.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close wal")
		}
	}()

	app := newApplication(cfg, api.NewHandler(ex))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal().Err(err).Msg("server error")
		}
	case <-ctx.Done():
		if err := app.close(); err != nil {
			log.Error().Err(err).Msg("graceful shutdown error")
		}
	}

	log.Info().Msg("shutdown complete")
}
