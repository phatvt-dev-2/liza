package agent

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	// Create handler with custom options
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format to match state.yaml (ISO 8601 UTC)
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05Z"))
			}
			return a
		},
	}

	// Use TextHandler for human-readable output
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger = slog.New(handler)
}

// GetLogger returns the package logger
func GetLogger() *slog.Logger {
	return logger
}
