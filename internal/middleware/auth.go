package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type authMiddleware struct {
	AuthClient *auth.Client
	Log        *slog.Logger
}

func NewAuthMiddleware(client *auth.Client, log *slog.Logger) *authMiddleware {
	return &authMiddleware{
		AuthClient: client,
		Log:        log,
	}
}

// context key
type contextKey string

const UIDKey contextKey = "uid"
const EmailKey contextKey = "email"

// FirebaseAuth validates the Firebase token and adds user context to the logger
func (m *authMiddleware) FirebaseAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		header := r.Header.Get("Authorization")
		if header == "" {
			logger.FromContext(ctx).Warn("missing authorization header")
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			logger.FromContext(ctx).Warn("invalid authorization header format")
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenStr := parts[1]

		// Verify ID Token
		token, err := m.AuthClient.VerifyIDToken(ctx, tokenStr)
		if err != nil {
			logger.FromContext(ctx).Warn("token verification failed", "error", err)
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Enrich logger with user context
		email, _ := token.Claims["email"].(string)
		_, ctx = logger.With(ctx,
			"uid", token.UID,
			"email", email,
		)

		// Keep existing context values for backward compatibility
		ctx = context.WithValue(ctx, UIDKey, token.UID)
		ctx = context.WithValue(ctx, EmailKey, email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper to extract UID
func UID(ctx context.Context) string {
	uid, _ := ctx.Value(UIDKey).(string)
	return uid
}

// Helper to extract Email
func Email(ctx context.Context) string {
	uid, _ := ctx.Value(EmailKey).(string)
	return uid
}
