package handlers

import (
	"github.com/GregMSThompson/finance-backend/internal/errs"
)

type Deps struct {
	ErrorHandler errs.ErrorHandler
	UserSvc      UserService
}
