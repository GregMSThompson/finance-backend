package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/handlers"
)

func NewRouter(deps *handlers.Deps) chi.Router {
	r := chi.NewRouter()

	ush := handlers.NewUserHandlers(deps)

	r.Mount("/users", ush.UserRoutes())
	return r
}
