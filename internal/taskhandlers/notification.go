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

// notificationService delivers notifications across all supported channels.
type notificationService interface {
	SendPush(ctx context.Context, req dto.DeliverNotificationRequest) error
}

type notificationTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	NotificationSvc notificationService
}

// NewNotificationTaskHandlers creates handlers for notification delivery tasks.
func NewNotificationTaskHandlers(deps *Deps) *notificationTaskHandlers {
	return &notificationTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		NotificationSvc: deps.NotificationSvc,
	}
}

// Deliver dispatches a notification to the appropriate delivery channel.
// Called by Cloud Tasks; returns 2xx on success so the task is not retried.
func (h *notificationTaskHandlers) Deliver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req dto.DeliverNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid task payload", "error", err)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}

	if req.NotificationID == "" || req.UserID == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "notificationId and userId are required")
		return
	}

	if req.Delivery == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "delivery is required")
		return
	}

	var err error
	switch req.Delivery {
	case models.DeliveryPush:
		err = h.NotificationSvc.SendPush(ctx, req)
	default:
		log.Warn("unsupported delivery method", "delivery", req.Delivery)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "unsupported delivery method")
		return
	}

	if err != nil {
		log.Error("failed to deliver notification", "error", err, "notificationId", req.NotificationID, "delivery", req.Delivery)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "internal_error", "delivery failed")
		return
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}
