package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// jobStore is the minimal storage surface required by the job service.
type jobStore interface {
	Create(ctx context.Context, uid string, j *models.Job) error
	Get(ctx context.Context, uid, jobID string) (*models.Job, error)
	MarkRunning(ctx context.Context, uid, jobID string) error
	MarkCompleted(ctx context.Context, uid, jobID string, result json.RawMessage) error
	MarkFailed(ctx context.Context, uid, jobID, errMsg string) error
}

// taskEnqueuer dispatches a job to its worker route via Cloud Tasks.
type taskEnqueuer interface {
	EnqueueJob(ctx context.Context, jobType models.JobType, uid, jobID string) error
}

type jobService struct {
	store    jobStore
	enqueuer taskEnqueuer
}

func NewJobService(store jobStore, enqueuer taskEnqueuer) *jobService {
	return &jobService{store: store, enqueuer: enqueuer}
}

// Submit creates a pending job and enqueues a task. If enqueue fails the job is
// immediately marked failed so the user sees a clear error state.
func (s *jobService) Submit(ctx context.Context, uid string, jobType models.JobType, params json.RawMessage) (string, error) {
	if !jobType.IsValid() {
		return "", errs.NewValidationError("unknown job type")
	}

	jobID := uuid.New().String()
	job := &models.Job{
		JobID:  jobID,
		Type:   jobType,
		Status: models.JobStatusPending,
		Params: params,
	}
	if err := s.store.Create(ctx, uid, job); err != nil {
		return "", err
	}

	if err := s.enqueuer.EnqueueJob(ctx, jobType, uid, jobID); err != nil {
		_ = s.store.MarkFailed(ctx, uid, jobID, "failed to enqueue task")
		return "", err
	}
	return jobID, nil
}

func (s *jobService) Get(ctx context.Context, uid, jobID string) (*models.Job, error) {
	return s.store.Get(ctx, uid, jobID)
}

// Run loads the job, marks it running, executes work, and persists the outcome.
// Transient errors are returned without marking the job failed so Cloud Tasks can retry.
// Non-transient errors mark the job failed and are returned for the caller to translate
// to an ack-and-don't-retry HTTP response.
func (s *jobService) Run(ctx context.Context, uid, jobID string, work func(ctx context.Context, job *models.Job) (any, error)) error {
	log := logger.FromContext(ctx)

	job, err := s.store.Get(ctx, uid, jobID)
	if err != nil {
		return err
	}

	if err := s.store.MarkRunning(ctx, uid, jobID); err != nil {
		return err
	}

	result, workErr := work(ctx, job)
	if workErr != nil {
		if errs.IsTransient(workErr) {
			log.Warn("job transient failure; will retry", "job_id", jobID, "type", job.Type, "error", workErr)
			return workErr
		}
		if err := s.store.MarkFailed(ctx, uid, jobID, workErr.Error()); err != nil {
			log.Error("failed to mark job failed", "job_id", jobID, "error", err)
		}
		return workErr
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.store.MarkCompleted(ctx, uid, jobID, resultJSON)
}
