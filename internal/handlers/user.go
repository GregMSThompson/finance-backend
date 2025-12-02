package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/errs"
)

type UserService interface {
	CreateUser(ctx context.Context, uid, email, first, last string) error
}

type userHandlers struct {
	ErrorHander errs.ErrorHandler
	UserSvc     UserService
}

func NewUserHandlers(deps *Deps) *userHandlers {
	return &userHandlers{
		ErrorHander: deps.ErrorHandler,
		UserSvc:     deps.UserSvc,
	}
}

func (h *userHandlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateUser)
	return r
}

func (h *userHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		h.ErrorHander.HandleError(w, err)
		return
	}
}
