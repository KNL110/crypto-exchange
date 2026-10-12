// Package logging wires up the exchange's structured (zerolog) logging.
package logging

import (
	"os"
	"github.com/rs/zerolog"
)

// New builds the application-wide zerolog logger. Output is JSON to stdout
// by default; set pretty=true for a human-readable console writer during
// local development.
func New(level string, pretty bool) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	if pretty {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
