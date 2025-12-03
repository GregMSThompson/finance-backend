package handlers

import (
	"log/slog"

	"github.com/GregMSThompson/finance-backend/internal/response"
)

type Deps struct {
	Log             *slog.Logger
	ResponseHandler response.ResponseHandler
	UserSvc         UserService
}
