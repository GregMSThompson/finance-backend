package logger

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

func New(level slog.Level) *slog.Logger {
	handler := NewCloudRunHandler(level)
	return slog.New(handler)
}

// CloudRunHandler implements slog.Handler
type CloudRunHandler struct {
	level slog.Level
}

func NewCloudRunHandler(level slog.Level) *CloudRunHandler {
	return &CloudRunHandler{level: level}
}

func (h *CloudRunHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *CloudRunHandler) Handle(_ context.Context, r slog.Record) error {
	// Map slog levels → Cloud Logging severity
	severity := mapSeverity(r.Level)

	// Build the base event
	event := map[string]any{
		"severity": severity,
		"message":  r.Message,
		"time":     r.Time.Format(time.RFC3339Nano),
	}

	// Add attributes into event.data
	if r.NumAttrs() > 0 {
		data := make(map[string]any)

		r.Attrs(func(a slog.Attr) bool {
			data[a.Key] = a.Value.Any()
			return true
		})

		event["data"] = data
	}

	// Encode to JSON
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Cloud Run: stdout for all severities
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

func (h *CloudRunHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Add attributes to a new handler
	newH := *h
	return &withAttrsHandler{handler: &newH, attrs: attrs}
}

func (h *CloudRunHandler) WithGroup(_ string) slog.Handler {
	// CloudRun format doesn’t support groups — ignore
	return h
}

// ---- Helpers ----

func mapSeverity(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARNING"
	case slog.LevelError:
		return "ERROR"
	default:
		return "DEFAULT"
	}
}

// wrapper that injects static attrs
type withAttrsHandler struct {
	handler *CloudRunHandler
	attrs   []slog.Attr
}

func (h *withAttrsHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.handler.Enabled(ctx, l)
}

func (h *withAttrsHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, a := range h.attrs {
		r.AddAttrs(a)
	}
	return h.handler.Handle(ctx, r)
}

func (h *withAttrsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	all := append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &withAttrsHandler{handler: h.handler, attrs: all}
}

func (h *withAttrsHandler) WithGroup(name string) slog.Handler {
	return h
}
