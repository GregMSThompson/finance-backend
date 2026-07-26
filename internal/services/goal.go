package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type goalStore interface {
	Create(ctx context.Context, uid string, g *models.Goal) error
	Get(ctx context.Context, uid, goalID string) (*models.Goal, error)
	List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error)
	Update(ctx context.Context, uid string, g *models.Goal) error
	Delete(ctx context.Context, uid, goalID string) error
}

type goalSnapshotStore interface {
	Latest(ctx context.Context, uid, goalID string) (*models.GoalSnapshot, error)
	DeleteForGoal(ctx context.Context, uid, goalID string) error
}

type goalService struct {
	goals     goalStore
	snapshots goalSnapshotStore
	jobs      jobSubmitter
}

func NewGoalService(goals goalStore, snapshots goalSnapshotStore, jobs jobSubmitter) *goalService {
	return &goalService{goals: goals, snapshots: snapshots, jobs: jobs}
}

// Create validates the primitive and persists a new active goal, linking it to
// the chat session that created it.
func (s *goalService) Create(ctx context.Context, uid, sessionID string, def dto.GoalDefinition) (*models.Goal, error) {
	g := &models.Goal{
		GoalID:          uuid.NewString(),
		Type:            def.Type,
		Name:            def.Name,
		TargetValue:     def.TargetValue,
		TimeWindow:      def.TimeWindow,
		EndDate:         def.EndDate,
		Recurrence:      def.Recurrence,
		Filters:         def.Filters,
		AlertThresholds: def.AlertThresholds,
		Status:          models.GoalStatusActive,
		ConversationID:  sessionID,
	}
	if err := validateGoal(g); err != nil {
		return nil, err
	}
	if err := s.goals.Create(ctx, uid, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *goalService) Get(ctx context.Context, uid, goalID string) (*models.Goal, error) {
	return s.goals.Get(ctx, uid, goalID)
}

func (s *goalService) List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	return s.goals.List(ctx, uid, statuses...)
}

// Update applies a partial update, re-validating the merged goal. Status may
// only be moved between active and paused via the API — completed and failed
// are terminal outcomes set by evaluation.
func (s *goalService) Update(ctx context.Context, uid, goalID string, upd dto.GoalUpdate) (*models.Goal, error) {
	if upd.Status != nil {
		if err := validateStatusChange(*upd.Status); err != nil {
			return nil, err
		}
	}

	g, err := s.goals.Get(ctx, uid, goalID)
	if err != nil {
		return nil, err
	}

	applyGoalUpdate(g, upd)
	if err := validateGoal(g); err != nil {
		return nil, err
	}
	if err := s.goals.Update(ctx, uid, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Delete submits a goal-delete job and returns its ID. The actual cascade runs
// asynchronously on the worker via RunDelete.
func (s *goalService) Delete(ctx context.Context, uid, goalID string) (string, error) {
	params, err := json.Marshal(dto.GoalDeleteParams{GoalID: goalID})
	if err != nil {
		return "", err
	}
	return s.jobs.Submit(ctx, uid, models.JobTypeGoalDelete, params)
}

// RunDelete cascades the goal deletion: snapshots first, then the goal doc.
// Each step is idempotent so a retried job picks up where it left off.
func (s *goalService) RunDelete(ctx context.Context, uid string, params dto.GoalDeleteParams) (dto.GoalDeleteResult, error) {
	if err := s.snapshots.DeleteForGoal(ctx, uid, params.GoalID); err != nil {
		return dto.GoalDeleteResult{}, err
	}
	if err := s.goals.Delete(ctx, uid, params.GoalID); err != nil {
		return dto.GoalDeleteResult{}, err
	}
	return dto.GoalDeleteResult{GoalID: params.GoalID}, nil
}

// GetProgress returns the goal's latest evaluated progress. A goal that has not
// been evaluated yet returns zeroed progress with a zero AsOf — the client's
// "not measured yet" signal.
func (s *goalService) GetProgress(ctx context.Context, uid, goalID string) (dto.GoalProgress, error) {
	g, err := s.goals.Get(ctx, uid, goalID)
	if err != nil {
		return dto.GoalProgress{}, err
	}

	prog := dto.GoalProgress{
		GoalID:      g.GoalID,
		Name:        g.Name,
		Status:      g.Status,
		TargetValue: g.TargetValue,
	}

	snap, err := s.snapshots.Latest(ctx, uid, goalID)
	if err != nil {
		return dto.GoalProgress{}, err
	}
	if snap == nil {
		prog.AmountRemaining = g.TargetValue
		prog.IsOnTrack = true
		return prog, nil
	}

	prog.CurrentValue = snap.CurrentValue
	prog.AmountRemaining = g.TargetValue - snap.CurrentValue
	prog.PercentComplete = snap.PercentComplete
	prog.IsOnTrack = snap.IsOnTrack
	prog.AIInsight = snap.AIInsight
	prog.AsOf = snap.CreatedAt
	return prog, nil
}

func applyGoalUpdate(g *models.Goal, upd dto.GoalUpdate) {
	if upd.Name != nil {
		g.Name = *upd.Name
	}
	if upd.TargetValue != nil {
		g.TargetValue = *upd.TargetValue
	}
	if upd.TimeWindow != nil {
		g.TimeWindow = *upd.TimeWindow
	}
	if upd.EndDate != nil {
		g.EndDate = *upd.EndDate
	}
	if upd.Recurrence != nil {
		g.Recurrence = *upd.Recurrence
	}
	if upd.Filters != nil {
		g.Filters = *upd.Filters
	}
	if upd.AlertThresholds != nil {
		g.AlertThresholds = *upd.AlertThresholds
	}
	if upd.Status != nil {
		g.Status = *upd.Status
	}
}

// validateStatusChange guards the statuses a caller may set. completed and
// failed are set by the evaluator, not manually.
func validateStatusChange(status models.GoalStatus) error {
	switch status {
	case models.GoalStatusActive, models.GoalStatusPaused:
		return nil
	case models.GoalStatusCompleted, models.GoalStatusFailed:
		return errs.NewValidationError("completed and failed statuses are set by evaluation, not manually")
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid status: %s", status))
	}
}

// validateGoal enforces the Spending Limit primitive rules.
func validateGoal(g *models.Goal) error {
	if g.Type != models.GoalTypeSpendingLimit {
		return errs.NewValidationError(fmt.Sprintf("unsupported goal type: %s", g.Type))
	}
	if strings.TrimSpace(g.Name) == "" {
		return errs.NewValidationError("name is required")
	}
	if g.TargetValue <= 0 {
		return errs.NewValidationError("targetValue must be greater than 0")
	}

	switch g.Recurrence {
	case models.GoalRecurrenceRecurring:
		if g.TimeWindow != models.GoalWindowWeekly && g.TimeWindow != models.GoalWindowMonthly {
			return errs.NewValidationError("recurring goals must use a weekly or monthly window")
		}
	case models.GoalRecurrenceOneOff:
		// any window is valid for a one-off
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid recurrence: %s", g.Recurrence))
	}

	switch g.TimeWindow {
	case models.GoalWindowFixed:
		if g.EndDate == "" {
			return errs.NewValidationError("a fixed window requires an endDate")
		}
		if !helpers.IsValidDate(g.EndDate) {
			return errs.NewValidationError(fmt.Sprintf("invalid endDate: %s", g.EndDate))
		}
	case models.GoalWindowWeekly, models.GoalWindowMonthly:
		if g.EndDate != "" {
			return errs.NewValidationError("weekly and monthly windows must not set an endDate")
		}
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid timeWindow: %s", g.TimeWindow))
	}

	if g.Filters.PFCPrimary != "" && !taxonomy.IsPFCPrimaryAllowed(g.Filters.PFCPrimary) {
		return errs.NewValidationError(fmt.Sprintf("invalid category: %s", g.Filters.PFCPrimary))
	}

	if p := g.AlertThresholds.ProgressPercent; p != nil && (*p <= 0 || *p > 100) {
		return errs.NewValidationError("progressPercent must be between 0 and 100")
	}
	if st := g.AlertThresholds.SingleTransaction; st != nil && *st <= 0 {
		return errs.NewValidationError("singleTransaction must be greater than 0")
	}

	return nil
}
