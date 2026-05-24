package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type jobService interface {
	Get(ctx context.Context, uid, jobID string) (*models.Job, error)
}

type jobHandlers struct {
	ResponseHandler response.ResponseHandler
	JobSvc          jobService
}

func NewJobHandlers(deps *Deps) *jobHandlers {
	return &jobHandlers{
		ResponseHandler: deps.ResponseHandler,
		JobSvc:          deps.JobSvc,
	}
}

func (h *jobHandlers) JobRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{jobId}", h.GetJob)
	return r
}

func (h *jobHandlers) GetJob(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	jobID := chi.URLParam(r, "jobId")

	job, err := h.JobSvc.Get(r.Context(), uid, jobID)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, job)
}