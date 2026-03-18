package taskhandlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// notificationService sends push notifications for triggered alert events.
type notificationService interface {
	SendAlertPush(ctx context.Context, userID, alertEventID string) error
}

type alertTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	NotificationSvc notificationService
}

// NewAlertTaskHandlers creates handlers for alert delivery tasks.
func NewAlertTaskHandlers(deps *Deps) *alertTaskHandlers {
	return &alertTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		NotificationSvc: deps.NotificationSvc,
	}
}

type deliverAlertRequest struct {
	AlertEventID string `json:"alertEventId"`
	UserID       string `json:"userId"`
}

// DeliverAlertPush sends a push notification for a triggered alert event.
// Called by Cloud Tasks; returns 2xx on success so the task is not retried.
func (h *alertTaskHandlers) DeliverAlertPush(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req deliverAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid task payload", "error", err)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}

	if req.AlertEventID == "" || req.UserID == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "alertEventId and userId are required")
		return
	}

	if err := h.NotificationSvc.SendAlertPush(ctx, req.UserID, req.AlertEventID); err != nil {
		log.Error("failed to send alert push", "error", err, "alertEventId", req.AlertEventID)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "internal_error", "delivery failed")
		return
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}
