package logger

import (
	"context"
	"log/slog"
)

// contextKey is used to store the logger in context
type contextKey string

const loggerKey contextKey = "logger"

// ToContext stores a logger in the context
func ToContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from context.
// If no logger is found, returns the default logger.
// This ensures we never return nil.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// With is a convenience method that extracts the logger from context,
// adds attributes, and returns both the new logger and updated context.
// This makes it easy to add attributes in middleware:
//
//	logger, ctx := logger.With(ctx, "uid", uid, "email", email)
//	return handler.ServeHTTP(w, r.WithContext(ctx))
func With(ctx context.Context, args ...any) (*slog.Logger, context.Context) {
	logger := FromContext(ctx).With(args...)
	return logger, ToContext(ctx, logger)
}

// IsDebugEnabled checks if debug logging is enabled for the logger in context.
// Use this to conditionally perform expensive debug operations:
//
//	if logger.IsDebugEnabled(ctx) {
//	    // expensive debug data preparation
//	    logger.FromContext(ctx).Debug("debug info", "data", expensiveData)
//	}
func IsDebugEnabled(ctx context.Context) bool {
	logger := FromContext(ctx)
	return logger.Enabled(ctx, slog.LevelDebug)
}
