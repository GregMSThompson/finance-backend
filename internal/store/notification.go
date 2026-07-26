package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type notificationStore struct {
	client *firestore.Client
}

func NewNotificationStore(client *firestore.Client) *notificationStore {
	return &notificationStore{client: client}
}

func (s *notificationStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("notifications")
}

// Create writes a new notification. CreatedAt is set to now if zero.
func (s *notificationStore) Create(ctx context.Context, uid string, n *models.Notification) error {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	_, err := s.collection(uid).Doc(n.NotificationID).Set(ctx, n)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create notification", err)
	}
	return nil
}

// Get retrieves a notification by ID.
func (s *notificationStore) Get(ctx context.Context, uid, notificationID string) (*models.Notification, error) {
	doc, err := s.collection(uid).Doc(notificationID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("notification not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get notification", err)
	}
	var n models.Notification
	if err := doc.DataTo(&n); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse notification data", err)
	}
	return &n, nil
}

// ListRecent returns the most recent notifications for a user, newest first.
func (s *notificationStore) ListRecent(ctx context.Context, uid string, limit int) ([]*models.Notification, error) {
	docs, err := s.collection(uid).OrderBy("createdAt", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list notifications", err)
	}
	notifications := make([]*models.Notification, 0, len(docs))
	for _, doc := range docs {
		var n models.Notification
		if err := doc.DataTo(&n); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse notification data", err)
		}
		notifications = append(notifications, &n)
	}
	return notifications, nil
}

// MarkDelivered sets DeliveredAt to the current time.
func (s *notificationStore) MarkDelivered(ctx context.Context, uid, notificationID string) error {
	_, err := s.collection(uid).Doc(notificationID).Update(ctx, []firestore.Update{
		{Path: "deliveredAt", Value: time.Now()},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to mark notification delivered", err)
	}
	return nil
}
