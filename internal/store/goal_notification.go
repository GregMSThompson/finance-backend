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

type goalNotificationStore struct {
	client *firestore.Client
}

func NewGoalNotificationStore(client *firestore.Client) *goalNotificationStore {
	return &goalNotificationStore{client: client}
}

func (s *goalNotificationStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("goalNotifications")
}

// Create records a sent notification. NotificationID is the caller-computed
// dedup key (goal + trigger reason + period), so a batch retry re-Sets the same
// document rather than logging a duplicate.
func (s *goalNotificationStore) Create(ctx context.Context, uid string, n *models.GoalNotification) error {
	if n.SentAt.IsZero() {
		n.SentAt = time.Now()
	}
	_, err := s.collection(uid).Doc(n.NotificationID).Set(ctx, n)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create goal notification", err)
	}
	return nil
}

// Exists reports whether a notification with the given dedup key has already
// been recorded, so the evaluator can skip re-sending.
func (s *goalNotificationStore) Exists(ctx context.Context, uid, notificationID string) (bool, error) {
	_, err := s.collection(uid).Doc(notificationID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, errs.NewDatabaseError("read", "failed to check goal notification", err)
	}
	return true, nil
}
