package taskhandlers

import (
	"log/slog"

	"github.com/GregMSThompson/finance-backend/internal/response"
)

// Deps holds the dependencies shared across all task handlers.
type Deps struct {
	Log             *slog.Logger
	ResponseHandler response.ResponseHandler
	Audience        string // Cloud Tasks OIDC audience (worker service URL)
	TestAPIKey      string // non-empty enables test key bypass for local development
	NotificationSvc notificationService
}
