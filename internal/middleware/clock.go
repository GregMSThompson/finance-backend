package middleware

import (
	"net/http"
	"time"

	"github.com/GregMSThompson/finance-backend/pkg/clock"
)

// Clock attaches the system clock to each HTTP request context.
func Clock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(clock.WithClock(r.Context(), time.Now)))
	})
}
