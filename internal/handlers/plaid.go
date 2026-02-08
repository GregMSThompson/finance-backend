package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type plaidService interface {
	CreateLinkToken(ctx context.Context, uid string) (string, error)
	ExchangePublicToken(ctx context.Context, uid, publicToken, institutionName string) (string, error)
	SyncTransactions(ctx context.Context, uid string, bankID *string) (dto.PlaidServiceSyncResult, error)
}

type bankService interface {
	ListBanks(ctx context.Context, uid string) ([]*models.Bank, error)
	DeleteBank(ctx context.Context, uid, bankID string) error
}

type plaidHandlers struct {
	ResponseHandler response.ResponseHandler
	PlaidSvc        plaidService
	BankSvc         bankService
}

func NewPlaidHandlers(deps *Deps) *plaidHandlers {
	return &plaidHandlers{
		ResponseHandler: deps.ResponseHandler,
		PlaidSvc:        deps.PlaidSvc,
		BankSvc:         deps.BankSvc,
	}
}

func (h *plaidHandlers) PlaidRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/plaid/link-token", h.CreateLinkToken)
	r.Route("/banks", func(r chi.Router) {
		r.Post("/", h.LinkBank)
		r.Get("/", h.ListBanks)
		r.Delete("/{bankId}", h.DeleteBank)
	})
	r.Post("/transactions/sync", h.SyncTransactions)
	return r
}

func (h *plaidHandlers) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())

	linkToken, err := h.PlaidSvc.CreateLinkToken(r.Context(), uid)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, map[string]string{"linkToken": linkToken})
}

func (h *plaidHandlers) LinkBank(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicToken     string `json:"publicToken"`
		InstitutionName string `json:"institutionName,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	bankID, err := h.PlaidSvc.ExchangePublicToken(r.Context(), uid, body.PublicToken, body.InstitutionName)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, map[string]string{"bankId": bankID})
}

func (h *plaidHandlers) ListBanks(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())

	banks, err := h.BankSvc.ListBanks(r.Context(), uid)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, banks)
}

func (h *plaidHandlers) DeleteBank(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	bankID := chi.URLParam(r, "bankId")

	if err := h.BankSvc.DeleteBank(r.Context(), uid, bankID); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}

func (h *plaidHandlers) SyncTransactions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BankID *string `json:"bankId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && errors.Is(err, io.EOF) { // allow empty body
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	result, err := h.PlaidSvc.SyncTransactions(r.Context(), uid, body.BankID)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, result)
}
