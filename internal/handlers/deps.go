package handlers

import (
	"log/slog"

	"firebase.google.com/go/v4/auth"

	plaidclient "github.com/GregMSThompson/finance-backend/internal/client/plaid"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type Deps struct {
	Log             *slog.Logger
	ResponseHandler response.ResponseHandler
	Firebase        *auth.Client
	PlaidVerifier   *plaidclient.Verifier
	AppleAppID      string
	UserSvc         userService
	PlaidSvc        plaidService
	PlaidWebhookSvc plaidWebhookService
	BankSvc         bankService
	AISvc           aiService
	DashboardSvc    dashboardService
	AlertSvc        alertService
	GoalSvc         goalService
	JobSvc          jobService
	TransactionsSvc transactionsService
	AccountsSvc     accountsService
}
