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

type alertEventStore struct {
	client *firestore.Client
}

// NewAlertEventStore creates an alert event store backed by Firestore.
func NewAlertEventStore(client *firestore.Client) *alertEventStore {
	return &alertEventStore{client: client}
}

// collection returns the alert_events subcollection for a given user.
func (s *alertEventStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("alert_events")
}

// Create writes a new alert event. TriggeredAt is set to now if zero.
func (s *alertEventStore) Create(ctx context.Context, uid string, e *models.AlertEvent) error {
	if e.TriggeredAt.IsZero() {
		e.TriggeredAt = time.Now()
	}
	_, err := s.collection(uid).Doc(e.EventID).Set(ctx, e)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create alert event", err)
	}
	return nil
}

// Get retrieves an alert event by ID.
func (s *alertEventStore) Get(ctx context.Context, uid, eventID string) (*models.AlertEvent, error) {
	doc, err := s.collection(uid).Doc(eventID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("alert event not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get alert event", err)
	}
	var e models.AlertEvent
	if err := doc.DataTo(&e); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse alert event data", err)
	}
	return &e, nil
}

// MarkDelivered sets DeliveredAt to the current time.
func (s *alertEventStore) MarkDelivered(ctx context.Context, uid, eventID string) error {
	now := time.Now()
	_, err := s.collection(uid).Doc(eventID).Update(ctx, []firestore.Update{
		{Path: "deliveredAt", Value: now},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to mark alert event delivered", err)
	}
	return nil
}
