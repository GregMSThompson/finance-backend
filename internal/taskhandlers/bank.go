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

// bankRunner cascades a bank deletion: transactions, cursor, then the bank doc.
type bankRunner interface {
	RunDelete(ctx context.Context, uid string, params dto.BankDeleteParams) (dto.BankDeleteResult, error)
}

type bankTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	JobSvc          jobRunner
	BankSvc         bankRunner
}

func NewBankTaskHandlers(deps *Deps) *bankTaskHandlers {
	return &bankTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		JobSvc:          deps.JobSvc,
		BankSvc:         deps.BankSvc,
	}
}

// Delete runs a bank.delete job. The closure unmarshals job.Params and calls
// bankSvc.RunDelete; jobSvc.Run handles state transitions and retry classification.
func (h *bankTaskHandlers) Delete(w http.ResponseWriter, r *http.Request) {
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
		var params dto.BankDeleteParams
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return nil, err
		}
		return h.BankSvc.RunDelete(ctx, body.UID, params)
	})

	if err != nil {
		if errs.IsTransient(err) {
			log.Warn("bank delete transient failure", "job_id", body.JobID, "error", err)
			h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "transient", "retryable failure")
			return
		}
		log.Error("bank delete failed", "job_id", body.JobID, "error", err)
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}