package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/internal/handlers"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
)

func NewAPIRouter(deps *handlers.Deps) chi.Router {
	r := chi.NewRouter()

	// global middleware (applied to every route)
	loggerMw := middleware.NewLoggerMiddleware(deps.Log)
	authMw := middleware.NewAuthMiddleware(deps.Firebase, deps.ResponseHandler)
	plaidWebhookMw := middleware.NewPlaidWebhookMiddleware(deps.PlaidVerifier, deps.ResponseHandler)

	r.Use(chimiddleware.RequestID)
	r.Use(loggerMw.LoggerMiddleware)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// handlers
	ush := handlers.NewUserHandlers(deps)
	ph := handlers.NewPlaidHandlers(deps)
	pwh := handlers.NewPlaidWebhookHandlers(deps)
	bh := handlers.NewBankHandlers(deps)
	aih := handlers.NewAIHandlers(deps)
	dsh := handlers.NewDashboardHandlers(deps)
	alh := handlers.NewAlertHandlers(deps)
	jh := handlers.NewJobHandlers(deps)

	// public group — authenticated by Plaid's JWS, not Firebase
	r.Group(func(r chi.Router) {
		r.Use(plaidWebhookMw.VerifySignature)
		r.Mount("/plaid/webhook", pwh.WebhookRoutes())
	})

	// authenticated group — everything user-facing
	r.Group(func(r chi.Router) {
		r.Use(authMw.FirebaseAuth)
		r.Mount("/users", ush.UserRoutes())
		r.Mount("/plaid", ph.PlaidRoutes())
		r.Mount("/banks", bh.BankRoutes())
		r.Mount("/ai", aih.AIRoutes())
		r.Mount("/dashboard", dsh.DashboardRoutes())
		r.Mount("/alerts", alh.AlertRoutes())
		r.Mount("/jobs", jh.JobRoutes())
	})

	return r
}
