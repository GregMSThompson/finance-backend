package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

const (
	goalSnapshotsDefaultLimit = 90
	goalSnapshotsMaxLimit     = 366
)

type goalService interface {
	List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error)
	Get(ctx context.Context, uid, goalID string) (*models.Goal, error)
	GetProgress(ctx context.Context, uid, goalID string) (dto.GoalProgress, error)
	ListSnapshots(ctx context.Context, uid, goalID string, limit int) ([]*models.GoalSnapshot, error)
	ListGoalTransactions(ctx context.Context, uid, goalID string, cursor *string, limit int) (dto.TransactionListResult, error)
	Update(ctx context.Context, uid, goalID string, upd dto.GoalUpdate) (*models.Goal, error)
	Delete(ctx context.Context, uid, goalID string) (string, error)
}

type goalHandlers struct {
	ResponseHandler response.ResponseHandler
	GoalSvc         goalService
}

func NewGoalHandlers(deps *Deps) *goalHandlers {
	return &goalHandlers{
		ResponseHandler: deps.ResponseHandler,
		GoalSvc:         deps.GoalSvc,
	}
}

func (h *goalHandlers) GoalRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetGoals)
	r.Get("/{goalId}", h.GetGoal)
	r.Get("/{goalId}/progress", h.GetGoalProgress)
	r.Get("/{goalId}/snapshots", h.GetGoalSnapshots)
	r.Get("/{goalId}/transactions", h.GetGoalTransactions)
	r.Patch("/{goalId}", h.PatchGoal)
	r.Delete("/{goalId}", h.DeleteGoal)
	return r
}

func (h *goalHandlers) GetGoals(w http.ResponseWriter, r *http.Request) {
	statuses, err := parseGoalStatuses(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	goals, err := h.GoalSvc.List(r.Context(), uid, statuses...)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, goals)
}

func (h *goalHandlers) GetGoal(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	goal, err := h.GoalSvc.Get(r.Context(), uid, chi.URLParam(r, "goalId"))
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, goal)
}

func (h *goalHandlers) GetGoalProgress(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	progress, err := h.GoalSvc.GetProgress(r.Context(), uid, chi.URLParam(r, "goalId"))
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, progress)
}

func (h *goalHandlers) GetGoalSnapshots(w http.ResponseWriter, r *http.Request) {
	limit, err := parseGoalSnapshotsLimit(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	snapshots, err := h.GoalSvc.ListSnapshots(r.Context(), uid, chi.URLParam(r, "goalId"), limit)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, snapshots)
}

func (h *goalHandlers) GetGoalTransactions(w http.ResponseWriter, r *http.Request) {
	limit, err := parseGoalTransactionsLimit(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	cursor := helpers.OptString(r.URL.Query().Get("cursor"))
	uid := middleware.UID(r.Context())
	result, err := h.GoalSvc.ListGoalTransactions(r.Context(), uid, chi.URLParam(r, "goalId"), cursor, limit)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *goalHandlers) PatchGoal(w http.ResponseWriter, r *http.Request) {
	var req dto.PatchGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	// Map the narrow REST body onto a partial update — status and thresholds
	// only. Substantive fields are unreachable from this endpoint by design.
	upd := dto.GoalUpdate{
		Status:          req.Status,
		AlertThresholds: req.AlertThresholds,
	}
	uid := middleware.UID(r.Context())
	goal, err := h.GoalSvc.Update(r.Context(), uid, chi.URLParam(r, "goalId"), upd)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, goal)
}

func (h *goalHandlers) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	jobID, err := h.GoalSvc.Delete(r.Context(), uid, chi.URLParam(r, "goalId"))
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusAccepted, dto.SubmitJobResponse{JobID: jobID})
}

// parseGoalStatuses reads the repeated ?status= filter, defaulting to the
// active + paused set shown on the Goals tab.
func parseGoalStatuses(r *http.Request) ([]models.GoalStatus, error) {
	raw := r.URL.Query()["status"]
	if len(raw) == 0 {
		return []models.GoalStatus{models.GoalStatusActive, models.GoalStatusPaused}, nil
	}
	statuses := make([]models.GoalStatus, 0, len(raw))
	for _, s := range raw {
		st := models.GoalStatus(s)
		switch st {
		case models.GoalStatusActive, models.GoalStatusPaused, models.GoalStatusCompleted, models.GoalStatusFailed:
			statuses = append(statuses, st)
		default:
			return nil, errs.NewValidationError(fmt.Sprintf("invalid status: %s", s))
		}
	}
	return statuses, nil
}

// parseGoalTransactionsLimit reuses the GET /transactions paging bounds so a
// goal's transaction list behaves identically to the main list.
func parseGoalTransactionsLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return listTransactionsDefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errs.NewValidationError("limit must be a positive integer")
	}
	if n > listTransactionsMaxLimit {
		n = listTransactionsMaxLimit
	}
	return n, nil
}

func parseGoalSnapshotsLimit(r *http.Request) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return goalSnapshotsDefaultLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, errs.NewValidationError("limit must be a positive integer")
	}
	if limit > goalSnapshotsMaxLimit {
		limit = goalSnapshotsMaxLimit
	}
	return limit, nil
}
