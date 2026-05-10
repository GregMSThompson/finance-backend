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
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type alertService interface {
	CreateAlert(ctx context.Context, uid string, req dto.CreateAlertRequest) (*models.Alert, error)
	GetAlerts(ctx context.Context, uid string) ([]*models.Alert, error)
	GetAlertHistory(ctx context.Context, uid string, limit int) ([]*models.AlertEvent, error)
	UpdateAlert(ctx context.Context, uid, alertID string, req dto.UpdateAlertRequest) (*models.Alert, error)
	DeleteAlert(ctx context.Context, uid, alertID string) error
}

type alertHandlers struct {
	ResponseHandler response.ResponseHandler
	AlertSvc        alertService
}

func NewAlertHandlers(deps *Deps) *alertHandlers {
	return &alertHandlers{
		ResponseHandler: deps.ResponseHandler,
		AlertSvc:        deps.AlertSvc,
	}
}

func (h *alertHandlers) AlertRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAlerts)
	r.Get("/history", h.GetAlertHistory)
	r.Post("/", h.CreateAlert)
	r.Put("/{alertId}", h.UpdateAlert)
	r.Delete("/{alertId}", h.DeleteAlert)
	return r
}

func (h *alertHandlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	alerts, err := h.AlertSvc.GetAlerts(r.Context(), uid)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, alerts)
}

func (h *alertHandlers) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit, err := parseAlertHistoryLimit(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	events, err := h.AlertSvc.GetAlertHistory(r.Context(), uid, limit)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, events)
}

func (h *alertHandlers) CreateAlert(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	alert, err := h.AlertSvc.CreateAlert(r.Context(), uid, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusCreated, alert)
}

func (h *alertHandlers) UpdateAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertId")
	var req dto.UpdateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	alert, err := h.AlertSvc.UpdateAlert(r.Context(), uid, alertID, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, alert)
}

func (h *alertHandlers) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertId")
	uid := middleware.UID(r.Context())
	if err := h.AlertSvc.DeleteAlert(r.Context(), uid, alertID); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}

func parseAlertHistoryLimit(r *http.Request) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return 0, nil
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, errs.NewValidationError("limit must be a positive integer")
	}
	if limit <= 0 {
		return 0, errs.NewValidationError("limit must be a positive integer")
	}

	return limit, nil
}
