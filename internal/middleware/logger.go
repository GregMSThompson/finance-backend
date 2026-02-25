package middleware

import (
	"log/slog"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type loggerMiddleware struct {
	Log *slog.Logger
}

func NewLoggerMiddleware(log *slog.Logger) *loggerMiddleware {
	return &loggerMiddleware{Log: log}
}

// LoggerMiddleware initializes a request-scoped logger with request context.
// This should be one of the first middlewares in the chain.
func (m *loggerMiddleware) LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract request ID from Chi middleware (if present)
		requestID := chimiddleware.GetReqID(r.Context())

		// Create logger with request-level attributes
		enrichedLogger := m.Log.With(
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
		)

		// Add logger to context
		ctx := logger.ToContext(r.Context(), enrichedLogger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
