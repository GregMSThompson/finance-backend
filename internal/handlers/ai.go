package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

const (
	conversationDefaultLimit = 100
	conversationMaxLimit     = 500
)

type aiService interface {
	Query(ctx context.Context, uid string, req dto.AIQueryRequest) (dto.AIQueryResponse, error)
	GetConversation(ctx context.Context, uid, sessionID string, limit int) (dto.AIConversationResponse, error)
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
	r.Get("/conversations/{sessionId}", h.GetConversation)
	return r
}

func (h *aiHandlers) Query(w http.ResponseWriter, r *http.Request) {
	// TODO: Add request-scoped timeouts and per-resource timeouts once latency budgets are defined.
	var req dto.AIQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	if req.Message == "" {
		h.ResponseHandler.HandleError(w, r, errs.NewValidationError("message is required"))
		return
	}
	if req.SessionID == "" {
		h.ResponseHandler.HandleError(w, r, errs.NewValidationError("sessionId is required"))
		return
	}

	uid := middleware.UID(r.Context())
	resp, err := h.AISvc.Query(r.Context(), uid, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, resp)
}

func (h *aiHandlers) GetConversation(w http.ResponseWriter, r *http.Request) {
	limit, err := parseConversationLimit(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	resp, err := h.AISvc.GetConversation(r.Context(), uid, chi.URLParam(r, "sessionId"), limit)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, resp)
}

func parseConversationLimit(r *http.Request) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return conversationDefaultLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, errs.NewValidationError("limit must be a positive integer")
	}
	if limit > conversationMaxLimit {
		limit = conversationMaxLimit
	}
	return limit, nil
}
