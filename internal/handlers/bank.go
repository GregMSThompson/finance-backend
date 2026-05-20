package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type bankService interface {
	ListBanks(ctx context.Context, uid string) ([]*models.Bank, error)
	DeleteBank(ctx context.Context, uid, bankID string) error
}

type bankHandlers struct {
	ResponseHandler response.ResponseHandler
	BankSvc         bankService
	PlaidSvc        plaidService
}

func NewBankHandlers(deps *Deps) *bankHandlers {
	return &bankHandlers{
		ResponseHandler: deps.ResponseHandler,
		BankSvc:         deps.BankSvc,
		PlaidSvc:        deps.PlaidSvc,
	}
}

func (h *bankHandlers) BankRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.LinkBank)
	r.Get("/", h.ListBanks)
	r.Delete("/{bankId}", h.DeleteBank)
	return r
}

func (h *bankHandlers) LinkBank(w http.ResponseWriter, r *http.Request) {
	var req dto.LinkBankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	bankID, err := h.PlaidSvc.ExchangePublicToken(r.Context(), uid, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, map[string]string{"bankId": bankID})
}

func (h *bankHandlers) ListBanks(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())

	banks, err := h.BankSvc.ListBanks(r.Context(), uid)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, banks)
}

func (h *bankHandlers) DeleteBank(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	bankID := chi.URLParam(r, "bankId")

	if err := h.BankSvc.DeleteBank(r.Context(), uid, bankID); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}