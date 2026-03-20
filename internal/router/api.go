package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/internal/handlers"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
)

func NewAPIRouter(deps *handlers.Deps) chi.Router {
	r := chi.NewRouter()

	// middleware
	loggerMw := middleware.NewLoggerMiddleware(deps.Log)
	authMw := middleware.NewAuthMiddleware(deps.Firebase, deps.ResponseHandler)

	r.Use(chimiddleware.RequestID)   // 1. Generate request_id
	r.Use(loggerMw.LoggerMiddleware) // 2. Add logger with request context
	r.Use(authMw.FirebaseAuth)       // 3. Add user context to logger
	r.Use(chimiddleware.Logger)      // 4. Chi's HTTP logging
	r.Use(chimiddleware.Recoverer)   // 5. Panic recovery

	// handlers
	ush := handlers.NewUserHandlers(deps)
	ph := handlers.NewPlaidHandlers(deps)
	aih := handlers.NewAIHandlers(deps)
	dsh := handlers.NewDashboardHandlers(deps)
	alh := handlers.NewAlertHandlers(deps)

	r.Mount("/users", ush.UserRoutes())
	r.Mount("/", ph.PlaidRoutes())
	r.Mount("/ai", aih.AIRoutes())
	r.Mount("/dashboard", dsh.DashboardRoutes())
	r.Mount("/alerts", alh.AlertRoutes())
	return r
}
