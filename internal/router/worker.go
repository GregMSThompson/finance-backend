package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/taskhandlers"
)

func NewWorkerRouter(deps *taskhandlers.Deps) chi.Router {
	r := chi.NewRouter()

	// middleware
	loggerMw := middleware.NewLoggerMiddleware(deps.Log)
	ctMw := middleware.NewCloudTasksMiddleware(deps.Audience, deps.AppEnv, deps.TestAPIKey, deps.TestAPIKeyEnabled, deps.ResponseHandler)

	r.Use(chimiddleware.RequestID)
	r.Use(loggerMw.LoggerMiddleware)
	r.Use(ctMw.CloudTasksAuth)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// handlers
	nh := taskhandlers.NewNotificationTaskHandlers(deps)
	pth := taskhandlers.NewPlaidTaskHandlers(deps)
	bth := taskhandlers.NewBankTaskHandlers(deps)
	ath := taskhandlers.NewAccountsTaskHandlers(deps)

	r.Post("/tasks/notification-deliver", nh.Deliver)
	r.Post("/tasks/plaid/sync", pth.Sync)
	r.Post("/tasks/bank/delete", bth.Delete)
	r.Post("/tasks/account/sync", ath.Sync)

	return r
}
