package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// plaidWebhookVerifier is the narrow surface this middleware needs from the
// verifier. Satisfied by *plaidclient.Verifier; declared here so the middleware
// stays unaware of the SDK package.
type plaidWebhookVerifier interface {
	Verify(ctx context.Context, header string, body []byte) error
}

type plaidWebhookMiddleware struct {
	Verifier plaidWebhookVerifier
	Response response.ResponseHandler
}

// NewPlaidWebhookMiddleware creates middleware that authenticates inbound Plaid
// webhook requests by validating the Plaid-Verification JWS against a Plaid-hosted
// public key.
func NewPlaidWebhookMiddleware(v plaidWebhookVerifier, rh response.ResponseHandler) *plaidWebhookMiddleware {
	return &plaidWebhookMiddleware{Verifier: v, Response: rh}
}

// VerifySignature reads the request body, validates the signature, and restores
// the body so the downstream handler can decode it.
func (m *plaidWebhookMiddleware) VerifySignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.Response.WriteError(w, r, http.StatusBadRequest, "bad_request", "failed to read request body")
			return
		}
		_ = r.Body.Close()

		if err := m.Verifier.Verify(r.Context(), r.Header.Get("Plaid-Verification"), body); err != nil {
			logger.FromContext(r.Context()).Warn("plaid webhook verification failed", "error", err)
			m.Response.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "invalid webhook signature")
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(w, r)
	})
}