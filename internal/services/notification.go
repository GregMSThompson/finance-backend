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

type notificationStore interface {
	Get(ctx context.Context, uid, notificationID string) (*models.Notification, error)
	MarkDelivered(ctx context.Context, uid, notificationID string) error
}

type fcmClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

type notificationService struct {
	userStore     notificationUserStore
	notifications notificationStore
	fcm           fcmClient
}

// NewNotificationService creates a service for delivering notifications.
func NewNotificationService(
	userStore notificationUserStore,
	notifications notificationStore,
	fcm fcmClient,
) *notificationService {
	return &notificationService{
		userStore:     userStore,
		notifications: notifications,
		fcm:           fcm,
	}
}

// SendPush delivers a notification via FCM push. It fetches the user's FCM token
// and the pre-built message content from the notification. If the notification
// has already been delivered, it returns nil — this guards against duplicate
// sends on Cloud Tasks retry. Source-agnostic: alerts and goals deliver identically.
func (s *notificationService) SendPush(ctx context.Context, req dto.DeliverNotificationRequest) error {
	log := logger.FromContext(ctx)

	user, err := s.userStore.GetUser(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user.FCMToken == "" {
		return errs.NewValidationError("user has no FCM token registered")
	}

	notification, err := s.notifications.Get(ctx, req.UserID, req.NotificationID)
	if err != nil {
		return fmt.Errorf("get notification: %w", err)
	}

	// Guard against duplicate delivery on retry.
	if notification.DeliveredAt != nil {
		log.Info("notification already delivered, skipping", "notificationId", req.NotificationID)
		return nil
	}

	_, err = s.fcm.Send(ctx, &messaging.Message{
		Token: user.FCMToken,
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Data: notification.Data,
	})
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}

	if err := s.notifications.MarkDelivered(ctx, req.UserID, req.NotificationID); err != nil {
		// Delivery succeeded — log and continue rather than returning an error
		// that would cause the task to retry and re-send the notification.
		log.Warn("failed to mark notification delivered", "notificationId", req.NotificationID, "error", err)
	}

	return nil
}
