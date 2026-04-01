package middleware

import (
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"

	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type cloudTasksMiddleware struct {
	Audience          string
	AppEnv            string
	TestAPIKey        string
	TestAPIKeyEnabled bool
	Response          response.ResponseHandler
}

// NewCloudTasksMiddleware creates middleware that authenticates inbound Cloud Tasks requests
// via Google OIDC token verification. If the test key bypass is enabled, requests bearing
// that key are accepted without OIDC validation to support local development.
func NewCloudTasksMiddleware(audience, appEnv, testAPIKey string, testAPIKeyEnabled bool, rh response.ResponseHandler) *cloudTasksMiddleware {
	return &cloudTasksMiddleware{
		Audience:          audience,
		AppEnv:            appEnv,
		TestAPIKey:        testAPIKey,
		TestAPIKeyEnabled: testAPIKeyEnabled,
		Response:          rh,
	}
}

// CloudTasksAuth verifies the Google OIDC token added by Cloud Tasks to each dispatch request.
func (m *cloudTasksMiddleware) CloudTasksAuth(next http.Handler) http.Handler {
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

		token := parts[1]

		// Test key bypass is only active when explicitly enabled.
		if m.TestAPIKeyEnabled && m.AppEnv == "dev" && m.TestAPIKey != "" && token == m.TestAPIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Verify the Google OIDC token issued by Cloud Tasks.
		// The audience must match the worker service URL configured at deployment.
		if _, err := idtoken.Validate(ctx, token, m.Audience); err != nil {
			log.Warn("cloud tasks token verification failed", "error", err)
			m.Response.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
