package handlers

import (
	"log/slog"

	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/internal/response"
)

type Deps struct {
	Log             *slog.Logger
	ResponseHandler response.ResponseHandler
	UserSvc         UserService
	Firebase        *auth.Client
}
