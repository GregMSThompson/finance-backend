package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

type authMiddleware struct {
	AuthClient *auth.Client
}

func NewMiddleware(client *auth.Client) *authMiddleware {
	return &authMiddleware{AuthClient: client}
}

// context key
type contextKey string

const UIDKey contextKey = "uid"
const EmailKey contextKey = "email"

// Main middleware
func (m *authMiddleware) FirebaseAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenStr := parts[1]

		// Verify ID Token
		token, err := m.AuthClient.VerifyIDToken(r.Context(), tokenStr)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add UID and email to context
		email, _ := token.Claims["email"].(string)
		ctx := context.WithValue(r.Context(), UIDKey, token.UID)
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
