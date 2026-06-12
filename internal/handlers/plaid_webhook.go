package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type plaidWebhookService interface {
	HandleWebhook(ctx context.Context, payload dto.PlaidWebhook) error
}

type plaidWebhookHandlers struct {
	ResponseHandler response.ResponseHandler
	WebhookSvc      plaidWebhookService
}

func NewPlaidWebhookHandlers(deps *Deps) *plaidWebhookHandlers {
	return &plaidWebhookHandlers{
		ResponseHandler: deps.ResponseHandler,
		WebhookSvc:      deps.PlaidWebhookSvc,
	}
}

func (h *plaidWebhookHandlers) WebhookRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.HandleWebhook)
	return r
}

func (h *plaidWebhookHandlers) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload dto.PlaidWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	if err := h.WebhookSvc.HandleWebhook(r.Context(), payload); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}