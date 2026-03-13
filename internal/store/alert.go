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

type alertStore struct {
	client *firestore.Client
}

func NewAlertStore(client *firestore.Client) *alertStore {
	return &alertStore{client: client}
}

func (s *alertStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("alerts")
}

func (s *alertStore) Create(ctx context.Context, uid string, a *models.Alert) error {
	now := time.Now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	_, err := s.collection(uid).Doc(a.AlertID).Set(ctx, a)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create alert", err)
	}
	return nil
}

func (s *alertStore) Get(ctx context.Context, uid, alertID string) (*models.Alert, error) {
	doc, err := s.collection(uid).Doc(alertID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("alert not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get alert", err)
	}
	var a models.Alert
	if err := doc.DataTo(&a); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse alert data", err)
	}
	return &a, nil
}

func (s *alertStore) List(ctx context.Context, uid string) ([]*models.Alert, error) {
	docs, err := s.collection(uid).Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list alerts", err)
	}
	alerts := make([]*models.Alert, 0, len(docs))
	for _, d := range docs {
		var a models.Alert
		if err := d.DataTo(&a); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse alert data", err)
		}
		alerts = append(alerts, &a)
	}
	return alerts, nil
}

func (s *alertStore) Update(ctx context.Context, uid string, a *models.Alert) error {
	a.UpdatedAt = time.Now()
	_, err := s.collection(uid).Doc(a.AlertID).Set(ctx, a)
	if err != nil {
		return errs.NewDatabaseError("update", "failed to update alert", err)
	}
	return nil
}

func (s *alertStore) Delete(ctx context.Context, uid, alertID string) error {
	_, err := s.collection(uid).Doc(alertID).Delete(ctx)
	if err != nil {
		return errs.NewDatabaseError("delete", "failed to delete alert", err)
	}
	return nil
}
