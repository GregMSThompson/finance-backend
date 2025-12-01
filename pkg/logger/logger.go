package logger

import (
	"log/slog"
	"strings"
)

func New(level string, handler func(level slog.Level) slog.Handler) *slog.Logger {
	h := handler(getSlogLevel(level))
	return slog.New(h)
}

// ---- Helpers ----
func getSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
