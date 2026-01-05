package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type userService interface {
	CreateUser(ctx context.Context, uid, email, first, last string) error
}

type userHandlers struct {
	ResponseHandler response.ResponseHandler
	UserSvc         userService
}

func NewUserHandlers(deps *Deps) *userHandlers {
	return &userHandlers{
		ResponseHandler: deps.ResponseHandler,
		UserSvc:         deps.UserSvc,
	}
}

func (h *userHandlers) UserRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateUser)
	return r
}

func (h *userHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
	}

	ctx := r.Context()
	uid := middleware.UID(ctx)
	email := middleware.Email(ctx)

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		h.ResponseHandler.HandleError(w, err)
		return
	}

	err = h.UserSvc.CreateUser(r.Context(), uid, email, body.FirstName, body.LastName)
	if err != nil {
		h.ResponseHandler.HandleError(w, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, 200, nil)
}
