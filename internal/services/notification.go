package services

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/messaging"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type notificationUserStore interface {
	GetUser(ctx context.Context, uid string) (*models.User, error)
}

type notificationAlertEventStore interface {
	Get(ctx context.Context, uid, eventID string) (*models.AlertEvent, error)
	MarkDelivered(ctx context.Context, uid, eventID string) error
}

type fcmClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

type notificationService struct {
	userStore       notificationUserStore
	alertEventStore notificationAlertEventStore
	fcm             fcmClient
}

// NewNotificationService creates a service for delivering alert notifications.
func NewNotificationService(
	userStore notificationUserStore,
	alertEventStore notificationAlertEventStore,
	fcm fcmClient,
) *notificationService {
	return &notificationService{
		userStore:       userStore,
		alertEventStore: alertEventStore,
		fcm:             fcm,
	}
}

// SendAlertPush delivers a push notification for the given alert event.
// It fetches the user's FCM token and the pre-built message content from the event.
// If the event has already been delivered, it returns nil — this guards against
// duplicate sends on Cloud Tasks retry.
func (s *notificationService) SendAlertPush(ctx context.Context, req dto.DeliverAlertRequest) error {
	log := logger.FromContext(ctx)

	user, err := s.userStore.GetUser(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user.FCMToken == "" {
		return errs.NewValidationError("user has no FCM token registered")
	}

	event, err := s.alertEventStore.Get(ctx, req.UserID, req.AlertEventID)
	if err != nil {
		return fmt.Errorf("get alert event: %w", err)
	}

	// Guard against duplicate delivery on retry.
	if event.DeliveredAt != nil {
		log.Info("alert event already delivered, skipping", "alertEventId", req.AlertEventID)
		return nil
	}

	_, err = s.fcm.Send(ctx, &messaging.Message{
		Token: user.FCMToken,
		Notification: &messaging.Notification{
			Title: event.Title,
			Body:  event.Body,
		},
	})
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}

	if err := s.alertEventStore.MarkDelivered(ctx, req.UserID, req.AlertEventID); err != nil {
		// Delivery succeeded — log and continue rather than returning an error
		// that would cause the task to retry and re-send the notification.
		log.Warn("failed to mark alert event delivered", "alertEventId", req.AlertEventID, "error", err)
	}

	return nil
}
