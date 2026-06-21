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

type accountsRunner interface {
	RunSync(ctx context.Context, uid string, params dto.AccountSyncParams) (dto.AccountSyncResult, error)
}

type accountsTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	JobSvc          jobRunner
	AccountsSvc     accountsRunner
}

func NewAccountsTaskHandlers(deps *Deps) *accountsTaskHandlers {
	return &accountsTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		JobSvc:          deps.JobSvc,
		AccountsSvc:     deps.AccountsSvc,
	}
}

// Sync runs an account.sync job.
func (h *accountsTaskHandlers) Sync(w http.ResponseWriter, r *http.Request) {
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
		var params dto.AccountSyncParams
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return nil, err
		}
		return h.AccountsSvc.RunSync(ctx, body.UID, params)
	})

	if err != nil {
		if errs.IsTransient(err) {
			log.Warn("account sync transient failure", "job_id", body.JobID, "error", err)
			h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "transient", "retryable failure")
			return
		}
		log.Error("account sync failed", "job_id", body.JobID, "error", err)
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}
