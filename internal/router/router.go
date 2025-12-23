package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/GregMSThompson/finance-backend/internal/handlers"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
)

func NewRouter(deps *handlers.Deps) chi.Router {
	r := chi.NewRouter()

	// middleware
	auth := middleware.NewMiddleware(deps.Firebase)

	r.Use(auth.FirebaseAuth)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// handlers
	ush := handlers.NewUserHandlers(deps)

	r.Mount("/users", ush.UserRoutes())
	return r
}
