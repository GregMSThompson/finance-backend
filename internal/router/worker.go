package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/taskhandlers"
)

func NewWorkerRouter(deps *taskhandlers.Deps) chi.Router {
	r := chi.NewRouter()

	loggerMw := middleware.NewLoggerMiddleware(deps.Log)
	ctMw := middleware.NewCloudTasksMiddleware(deps.Audience, deps.TestAPIKey, deps.TestAPIKeyEnabled, deps.ResponseHandler)

	r.Use(chimiddleware.RequestID)
	r.Use(loggerMw.LoggerMiddleware)
	r.Use(ctMw.CloudTasksAuth)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	alh := taskhandlers.NewAlertTaskHandlers(deps)
	r.Post("/tasks/alert-deliver", alh.DeliverAlert)

	return r
}
