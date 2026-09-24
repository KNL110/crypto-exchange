package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/knl110/crypto-exchange/internal/config"
	"github.com/knl110/crypto-exchange/internal/exchange"
)

func main() {
	cfg := config.MustLoad()

	ex := exchange.NewExchange()
	// if err := ex.Recover(cfg.WALPath, []byte(cfg.WALHMACKey)); err != nil {
	// 	log.Fatal().Err(err).Msg("failed to recover exchange state from wal")
	// }
	// defer func() {
	// 	if err := ex.Close(); err != nil {
	// 		log.Error().Err(err).Msg("failed to close wal")
	// 	}
	// }()

	app := newApplication(cfg, exchange.NewHandler(ex))

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
