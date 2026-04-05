package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type userService interface {
	CreateUser(ctx context.Context, uid, email string, req dto.CreateUserRequest) error
	SetFCMToken(ctx context.Context, uid string, req dto.SetFCMTokenRequest) error
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
	r.Put("/fcm-token", h.SetFCMToken)
	return r
}

func (h *userHandlers) SetFCMToken(w http.ResponseWriter, r *http.Request) {
	var req dto.SetFCMTokenRequest

	ctx := r.Context()
	uid := middleware.UID(ctx)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	if err := h.UserSvc.SetFCMToken(ctx, uid, req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}

func (h *userHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest

	ctx := r.Context()
	uid := middleware.UID(ctx)
	email := middleware.Email(ctx)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	if err := h.UserSvc.CreateUser(ctx, uid, email, req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}
