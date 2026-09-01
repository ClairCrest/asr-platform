// Package observability wires structured logging and per-request context
// (request IDs, panic recovery) shared across the HTTP layer.
package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a JSON slog.Logger writing to stdout, suitable for
// container log collection.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
