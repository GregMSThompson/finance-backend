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
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type plaidService interface {
	CreateLinkToken(ctx context.Context, uid string) (string, error)
	ExchangePublicToken(ctx context.Context, uid string, req dto.LinkBankRequest) (string, error)
	SyncTransactions(ctx context.Context, uid string, req dto.SyncTransactionsRequest) (string, error)
}

type plaidHandlers struct {
	ResponseHandler response.ResponseHandler
	PlaidSvc        plaidService
}

func NewPlaidHandlers(deps *Deps) *plaidHandlers {
	return &plaidHandlers{
		ResponseHandler: deps.ResponseHandler,
		PlaidSvc:        deps.PlaidSvc,
	}
}

func (h *plaidHandlers) PlaidRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/link-token", h.CreateLinkToken)
	r.Post("/sync", h.SyncTransactions)
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

func (h *plaidHandlers) SyncTransactions(w http.ResponseWriter, r *http.Request) {
	var req dto.SyncTransactionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) { // allow empty body
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	jobID, err := h.PlaidSvc.SyncTransactions(r.Context(), uid, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusAccepted, dto.SubmitJobResponse{JobID: jobID})
}