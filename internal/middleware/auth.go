package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

type Middleware struct {
	AuthClient *auth.Client
}

func NewMiddleware(client *auth.Client) *Middleware {
	return &Middleware{AuthClient: client}
}

// context key
type contextKey string

const UIDKey contextKey = "uid"

// Main middleware
func (m *Middleware) FirebaseAuth(next http.Handler) http.Handler {
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

		// Add UID to context
		ctx := context.WithValue(r.Context(), UIDKey, token.UID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper to extract UID
func UID(ctx context.Context) string {
	uid, _ := ctx.Value(UIDKey).(string)
	return uid
}
