package dto

import (
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

// GoalDefinition is the validated primitive used to create a goal. Creation is
// chat-only (there is no HTTP create body), so this is a transport-neutral
// domain input rather than a Request. It decodes directly from the create_goal
// tool call.
type GoalDefinition struct {
	Type            models.GoalType            `json:"type"`
	Name            string                     `json:"name"`
	TargetValue     float64                    `json:"targetValue"`
	TimeWindow      models.GoalTimeWindow      `json:"timeWindow"`
	EndDate         string                     `json:"endDate,omitempty"`
	Recurrence      models.GoalRecurrence      `json:"recurrence"`
	Filters         models.GoalFilters         `json:"filters,omitempty"`
	AlertThresholds models.GoalAlertThresholds `json:"alertThresholds,omitempty"`
}

// GoalUpdate is a partial update to an existing goal — nil fields are left
// unchanged. It is fed by both the chat update_goal tool (substantive edits)
// and the REST PATCH endpoint (pause/resume, notification toggles), hence the
// pointer fields.
type GoalUpdate struct {
	Name            *string                     `json:"name,omitempty"`
	TargetValue     *float64                    `json:"targetValue,omitempty"`
	TimeWindow      *models.GoalTimeWindow      `json:"timeWindow,omitempty"`
	EndDate         *string                     `json:"endDate,omitempty"`
	Recurrence      *models.GoalRecurrence      `json:"recurrence,omitempty"`
	Filters         *models.GoalFilters         `json:"filters,omitempty"`
	AlertThresholds *models.GoalAlertThresholds `json:"alertThresholds,omitempty"`
	Status          *models.GoalStatus          `json:"status,omitempty"`
}

// PatchGoalRequest is the REST body for direct (non-chat) goal edits — status
// (pause/resume) and notification thresholds only. Substantive edits (target,
// window, category, name) go through the chat update_goal tool.
type PatchGoalRequest struct {
	Status          *models.GoalStatus          `json:"status,omitempty"`
	AlertThresholds *models.GoalAlertThresholds `json:"alertThresholds,omitempty"`
}

// GoalDeleteParams is the params payload for a JobTypeGoalDelete job.
type GoalDeleteParams struct {
	GoalID string `json:"goalId"`
}

// GoalDeleteResult summarises the outcome of a goal delete job.
type GoalDeleteResult struct {
	GoalID string `json:"goalId"`
}

// GoalProgress is the consistency-safe progress view returned by the
// get_goal_progress tool and the detail endpoint. It reflects the latest daily
// snapshot (AsOf signals staleness) rather than re-deriving live, so it
// always agrees with the number the notification quoted.
type GoalProgress struct {
	GoalID          string            `json:"goalId"`
	Name            string            `json:"name"`
	Status          models.GoalStatus `json:"status"`
	CurrentValue    float64           `json:"currentValue"`
	TargetValue     float64           `json:"targetValue"`
	AmountRemaining float64           `json:"amountRemaining"`
	PercentComplete float64           `json:"percentComplete"`
	IsOnTrack bool   `json:"isOnTrack"`
	AIInsight string `json:"aiInsight,omitempty"`
	// AsOf is the snapshot's CreatedAt — when this progress was last measured,
	// surfaced to the client as the staleness signal ("as of last night").
	AsOf time.Time `json:"asOf"`
}
