package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type accountsService interface {
	SyncAccounts(ctx context.Context, uid, bankID string) (string, error)
}

type accountsHandlers struct {
	ResponseHandler response.ResponseHandler
	AccountsSvc     accountsService
}

func NewAccountsHandlers(deps *Deps) *accountsHandlers {
	return &accountsHandlers{
		ResponseHandler: deps.ResponseHandler,
		AccountsSvc:     deps.AccountsSvc,
	}
}

func (h *accountsHandlers) AccountRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/sync", h.SyncAccounts)
	return r
}

func (h *accountsHandlers) SyncAccounts(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	bankID := chi.URLParam(r, "bankId")

	jobID, err := h.AccountsSvc.SyncAccounts(r.Context(), uid, bankID)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusAccepted, dto.SubmitJobResponse{JobID: jobID})
}
