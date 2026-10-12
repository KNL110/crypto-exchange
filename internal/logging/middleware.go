package logging

import (
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
)

// RequestLoggerMiddleware returns an Echo middleware that logs every
// completed request as a structured zerolog event.
func RequestLoggerMiddleware(log zerolog.Logger) echo.MiddlewareFunc {
	mw, err := middleware.RequestLoggerConfig{
		LogLatency:   true,
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogRemoteIP:  true,
		LogRequestID: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			evt := log.Info()
			if v.Error != nil || v.Status >= 500 {
				evt = log.Error()
			} else if v.Status >= 400 {
				evt = log.Warn()
			}
			evt.
				Str("method", v.Method).
				Str("uri", v.URI).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Str("remote_ip", v.RemoteIP).
				Str("request_id", v.RequestID).
				AnErr("error", v.Error).
				Msg("request")
			return nil
		},
	}.ToMiddleware()
	if err != nil {
		// ToMiddleware only errors on a missing LogValuesFunc, which we always
		// supply above, so this is unreachable in practice.
		panic(err)
	}
	return mw
}
