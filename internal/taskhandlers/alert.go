package taskhandlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// notificationService delivers alert notifications across all supported channels.
type notificationService interface {
	SendAlertPush(ctx context.Context, req dto.DeliverAlertRequest) error
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

// DeliverAlert dispatches an alert notification to the appropriate delivery channel.
// Called by Cloud Tasks; returns 2xx on success so the task is not retried.
func (h *alertTaskHandlers) DeliverAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req dto.DeliverAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid task payload", "error", err)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}

	if req.AlertEventID == "" || req.UserID == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "alertEventId and userId are required")
		return
	}

	if req.Delivery == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "delivery is required")
		return
	}

	var err error
	switch req.Delivery {
	case models.DeliveryPush:
		err = h.NotificationSvc.SendAlertPush(ctx, req)
	default:
		log.Warn("unsupported delivery method", "delivery", req.Delivery)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "unsupported delivery method")
		return
	}

	if err != nil {
		log.Error("failed to deliver alert", "error", err, "alertEventId", req.AlertEventID, "delivery", req.Delivery)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "internal_error", "delivery failed")
		return
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}
