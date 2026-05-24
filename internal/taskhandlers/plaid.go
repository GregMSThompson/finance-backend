package taskhandlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// jobRunner wraps job state transitions around a unit of work.
type jobRunner interface {
	Run(ctx context.Context, uid, jobID string, work func(ctx context.Context, job *models.Job) (any, error)) error
}

// plaidRunner performs the actual Plaid transaction sync.
type plaidRunner interface {
	RunSync(ctx context.Context, uid string, params dto.PlaidSyncParams) (dto.PlaidServiceSyncResult, error)
}

type plaidTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	JobSvc          jobRunner
	PlaidSvc        plaidRunner
}

func NewPlaidTaskHandlers(deps *Deps) *plaidTaskHandlers {
	return &plaidTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		JobSvc:          deps.JobSvc,
		PlaidSvc:        deps.PlaidSvc,
	}
}

// Sync runs a plaid.sync job. The job's params blob is decoded inside the work
// closure; jobService.Run handles state transitions around it.
func (h *plaidTaskHandlers) Sync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var body dto.JobTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("invalid task payload", "error", err)
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	if body.UID == "" || body.JobID == "" {
		h.ResponseHandler.WriteTaskError(w, r, http.StatusBadRequest, "bad_request", "uid and jobId are required")
		return
	}

	err := h.JobSvc.Run(ctx, body.UID, body.JobID, func(ctx context.Context, job *models.Job) (any, error) {
		var params dto.PlaidSyncParams
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return nil, err
		}
		return h.PlaidSvc.RunSync(ctx, body.UID, params)
	})

	if err != nil {
		// Transient errors return 5xx so Cloud Tasks retries.
		// Non-transient errors have already been recorded on the job; ack with 2xx
		// to prevent retry loops on permanent failures.
		if errs.IsTransient(err) {
			log.Warn("plaid sync transient failure", "job_id", body.JobID, "error", err)
			h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "transient", "retryable failure")
			return
		}
		log.Error("plaid sync failed", "job_id", body.JobID, "error", err)
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}