package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type authMiddleware struct {
	AuthClient *auth.Client
	Response   response.ResponseHandler
}

func NewAuthMiddleware(client *auth.Client, rh response.ResponseHandler) *authMiddleware {
	return &authMiddleware{
		AuthClient: client,
		Response:   rh,
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
		log := logger.FromContext(ctx)

		header := r.Header.Get("Authorization")
		if header == "" {
			log.Warn("missing authorization header")
			m.Response.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "missing Authorization header")
			return
		}

		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			log.Warn("invalid authorization header format")
			m.Response.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "invalid Authorization header")
			return
		}

		tokenStr := parts[1]

		// Verify ID Token
		token, err := m.AuthClient.VerifyIDToken(ctx, tokenStr)
		if err != nil {
			log.Warn("token verification failed", "error", err)
			m.Response.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		// Enrich logger with user context
		email, _ := token.Claims["email"].(string)
		_, ctx = logger.With(ctx,
			"uid", token.UID,
			"email", email,
		)

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
