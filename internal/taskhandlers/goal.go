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

// goalRunner cascades a goal deletion: snapshots, then the goal doc.
type goalRunner interface {
	RunDelete(ctx context.Context, uid string, params dto.GoalDeleteParams) (dto.GoalDeleteResult, error)
}

type goalTaskHandlers struct {
	ResponseHandler response.ResponseHandler
	JobSvc          jobRunner
	GoalSvc         goalRunner
}

func NewGoalTaskHandlers(deps *Deps) *goalTaskHandlers {
	return &goalTaskHandlers{
		ResponseHandler: deps.ResponseHandler,
		JobSvc:          deps.JobSvc,
		GoalSvc:         deps.GoalSvc,
	}
}

// Delete runs a goal.delete job. The closure unmarshals job.Params and calls
// goalSvc.RunDelete; jobSvc.Run handles state transitions and retry classification.
func (h *goalTaskHandlers) Delete(w http.ResponseWriter, r *http.Request) {
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
		var params dto.GoalDeleteParams
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return nil, err
		}
		return h.GoalSvc.RunDelete(ctx, body.UID, params)
	})

	if err != nil {
		if errs.IsTransient(err) {
			log.Warn("goal delete transient failure", "job_id", body.JobID, "error", err)
			h.ResponseHandler.WriteTaskError(w, r, http.StatusInternalServerError, "transient", "retryable failure")
			return
		}
		log.Error("goal delete failed", "job_id", body.JobID, "error", err)
	}

	h.ResponseHandler.WriteTaskSuccess(w, r)
}
