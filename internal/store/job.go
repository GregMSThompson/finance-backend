package store

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type jobStore struct {
	client *firestore.Client
}

func NewJobStore(client *firestore.Client) *jobStore {
	return &jobStore{client: client}
}

func (s *jobStore) Create(ctx context.Context, uid string, j *models.Job) error {
	now := time.Now()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	_, err := s.collection(uid).Doc(j.JobID).Set(ctx, j)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create job", err)
	}
	return nil
}

func (s *jobStore) Get(ctx context.Context, uid, jobID string) (*models.Job, error) {
	doc, err := s.collection(uid).Doc(jobID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("job not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get job", err)
	}
	var j models.Job
	if err := doc.DataTo(&j); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse job data", err)
	}
	return &j, nil
}

func (s *jobStore) MarkRunning(ctx context.Context, uid, jobID string) error {
	_, err := s.collection(uid).Doc(jobID).Update(ctx, []firestore.Update{
		{Path: "status", Value: models.JobStatusRunning},
		{Path: "updatedAt", Value: time.Now()},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to mark job running", err)
	}
	return nil
}

func (s *jobStore) MarkCompleted(ctx context.Context, uid, jobID string, result json.RawMessage) error {
	now := time.Now()
	_, err := s.collection(uid).Doc(jobID).Update(ctx, []firestore.Update{
		{Path: "status", Value: models.JobStatusCompleted},
		{Path: "result", Value: result},
		{Path: "updatedAt", Value: now},
		{Path: "completedAt", Value: now},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to mark job completed", err)
	}
	return nil
}

func (s *jobStore) MarkFailed(ctx context.Context, uid, jobID, errMsg string) error {
	now := time.Now()
	_, err := s.collection(uid).Doc(jobID).Update(ctx, []firestore.Update{
		{Path: "status", Value: models.JobStatusFailed},
		{Path: "error", Value: errMsg},
		{Path: "updatedAt", Value: now},
		{Path: "completedAt", Value: now},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to mark job failed", err)
	}
	return nil
}

func (s *jobStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("jobs")
}