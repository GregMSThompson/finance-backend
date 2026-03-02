package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type dashboardStore struct {
	client *firestore.Client
}

func NewDashboardStore(client *firestore.Client) *dashboardStore {
	return &dashboardStore{client: client}
}

func (s *dashboardStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("dashboard_widgets")
}

func (s *dashboardStore) Create(ctx context.Context, uid string, w *models.Widget) error {
	now := time.Now()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	w.UpdatedAt = now
	_, err := s.collection(uid).Doc(w.WidgetID).Set(ctx, w)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create widget", err)
	}
	return nil
}

func (s *dashboardStore) Get(ctx context.Context, uid, widgetID string) (*models.Widget, error) {
	doc, err := s.collection(uid).Doc(widgetID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("widget not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get widget", err)
	}
	var w models.Widget
	if err := doc.DataTo(&w); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse widget data", err)
	}
	return &w, nil
}

func (s *dashboardStore) List(ctx context.Context, uid string) ([]*models.Widget, error) {
	docs, err := s.collection(uid).OrderBy("position", firestore.Asc).Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list widgets", err)
	}
	widgets := make([]*models.Widget, 0, len(docs))
	for _, d := range docs {
		var w models.Widget
		if err := d.DataTo(&w); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse widget data", err)
		}
		widgets = append(widgets, &w)
	}
	return widgets, nil
}

func (s *dashboardStore) Update(ctx context.Context, uid string, w *models.Widget) error {
	w.UpdatedAt = time.Now()
	_, err := s.collection(uid).Doc(w.WidgetID).Set(ctx, w)
	if err != nil {
		return errs.NewDatabaseError("update", "failed to update widget", err)
	}
	return nil
}

func (s *dashboardStore) Delete(ctx context.Context, uid, widgetID string) error {
	_, err := s.collection(uid).Doc(widgetID).Delete(ctx)
	if err != nil {
		return errs.NewDatabaseError("delete", "failed to delete widget", err)
	}
	return nil
}

func (s *dashboardStore) Count(ctx context.Context, uid string) (int, error) {
	docs, err := s.collection(uid).Documents(ctx).GetAll()
	if err != nil {
		return 0, errs.NewDatabaseError("read", "failed to count widgets", err)
	}
	return len(docs), nil
}

type bulkPositionJob struct {
	widgetID string
	job      *firestore.BulkWriterJob
}

func (s *dashboardStore) BulkUpdatePositions(ctx context.Context, uid string, positions map[string]int) error {
	log := logger.FromContext(ctx)
	bw := s.client.BulkWriter(ctx)
	coll := s.collection(uid)
	now := time.Now()

	jobs := make([]bulkPositionJob, 0, len(positions))
	for widgetID, pos := range positions {
		ref := coll.Doc(widgetID)
		j, err := bw.Update(ref, []firestore.Update{
			{Path: "position", Value: pos},
			{Path: "updatedAt", Value: now},
		})
		if err != nil {
			return errs.NewDatabaseError("update", "failed to schedule position update", err)
		}
		jobs = append(jobs, bulkPositionJob{widgetID: widgetID, job: j})
	}
	bw.End()

	for _, entry := range jobs {
		if _, err := entry.job.Results(); err != nil {
			log.Error("failed to update widget position", "widget_id", entry.widgetID, "error", err)
			return errs.NewDatabaseError("update", "failed to update widget position", err)
		}
	}
	return nil
}
