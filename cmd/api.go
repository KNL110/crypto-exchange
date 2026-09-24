package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog/log"

	"github.com/knl110/crypto-exchange/internal/config"
	"github.com/knl110/crypto-exchange/internal/exchange"
)

type application struct {
	config   *config.Config
	exchange *exchange.Handler
	server   *http.Server
}

func newApplication(cfg *config.Config, exchangeHandler *exchange.Handler) *application {
	return &application{
		config:   cfg,
		exchange: exchangeHandler,
	}
}

// strictJSONSerializer is Echo's default JSON serializer, except that
// decoding rejects request bodies containing fields the target struct
// doesn't declare.
type strictJSONSerializer struct {
	echo.DefaultJSONSerializer
}

func (strictJSONSerializer) Deserialize(c *echo.Context, target any) error {
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}
	return nil
}

func (app *application) mount() *echo.Echo {
	e := echo.New()
	e.JSONSerializer = strictJSONSerializer{}

	api := e.Group("/api/v1")
	api.GET("/healthz", app.exchange.Health)

	exchange.RegisterRoutes(api.Group("/exchange"), app.exchange)

	return e
}

func (app *application) run() error {
	router := app.mount()

	app.server = &http.Server{
		Addr:         app.config.GetAddr(),
		Handler:      router,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	log.Info().Str("addr", app.server.Addr).Msg("starting exchange api")

	if err := app.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("app.run: server stopped unexpectedly -> %w", err)
	}
	return nil
}

func (app *application) close() error {
	log.Info().Msg("closing server...")

	timeout := app.config.ShutdownTimeout
	if timeout == 0 {
		timeout = time.Second * 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("app.close: failed to close server -> %w", err)
	}

	log.Info().Msg("server closed successfully")
	return nil
}
