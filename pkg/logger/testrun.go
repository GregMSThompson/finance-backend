package logger

import (
	"io"
	"log/slog"
)

func NewTestHandler(level slog.Level) slog.Handler {
	return slog.NewTextHandler(io.Discard, nil)
}
