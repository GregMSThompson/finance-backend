package handlers

import (
	"context"
	"log/slog"

	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/internal/response"
)

type Deps struct {
	Log              *slog.Logger
	ResponseHandler  response.ResponseHandler
	Firebase         *auth.Client
	PlaidVerifier    plaidWebhookVerifier
	UserSvc          userService
	PlaidSvc         plaidService
	PlaidWebhookSvc  plaidWebhookService
	BankSvc          bankService
	AISvc            aiService
	DashboardSvc     dashboardService
	AlertSvc         alertService
	JobSvc           jobService
}

// plaidWebhookVerifier is the narrow surface needed by the router to construct
// the Plaid webhook middleware. Satisfied by *plaidclient.Verifier.
type plaidWebhookVerifier interface {
	Verify(ctx context.Context, header string, body []byte) error
}
