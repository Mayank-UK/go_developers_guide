package core

import (
	"log/slog"
	"os"
)

// NewLogger returns the process logger: JSON records on stdout, gated at
// the configured level. JSON keeps every log line machine-parseable for
// the observability tooling later in the guide, and stdout is the
// container-native sink.
func NewLogger(cfg Config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
}
