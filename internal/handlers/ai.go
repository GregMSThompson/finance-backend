package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type aiService interface {
	Query(ctx context.Context, uid, sessionID, message string) (dto.AIQueryResponse, error)
}

type aiHandlers struct {
	ResponseHandler response.ResponseHandler
	AISvc           aiService
}

func NewAIHandlers(deps *Deps) *aiHandlers {
	return &aiHandlers{
		ResponseHandler: deps.ResponseHandler,
		AISvc:           deps.AISvc,
	}
}

func (h *aiHandlers) AIRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/query", h.Query)
	return r
}

func (h *aiHandlers) Query(w http.ResponseWriter, r *http.Request) {
	var body dto.AIQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.ResponseHandler.HandleError(w, err)
		return
	}
	if body.Message == "" {
		h.ResponseHandler.HandleError(w, errs.NewValidationError("message is required"))
		return
	}
	if body.SessionID == "" {
		h.ResponseHandler.HandleError(w, errs.NewValidationError("sessionId is required"))
		return
	}

	uid := middleware.UID(r.Context())
	resp, err := h.AISvc.Query(r.Context(), uid, body.SessionID, body.Message)
	if err != nil {
		h.ResponseHandler.HandleError(w, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, http.StatusOK, resp)
}
