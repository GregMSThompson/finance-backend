package models

import (
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// GoalType identifies the kind of target a goal tracks.
type GoalType string

const (
	GoalTypeSpendingLimit GoalType = "spending_limit"
	GoalTypeReduction     GoalType = "reduction"
	// Further types (savings_target, pay_down, ...) are added as their
	// evaluation logic lands.
)

// GoalTimeWindow is the period a goal is measured over.
type GoalTimeWindow string

const (
	GoalWindowWeekly  GoalTimeWindow = "weekly"
	GoalWindowMonthly GoalTimeWindow = "monthly"
	GoalWindowFixed   GoalTimeWindow = "fixed" // bounded by EndDate
)

// GoalRecurrence is whether a goal resets each period or runs once to an end date.
type GoalRecurrence string

const (
	GoalRecurrenceRecurring GoalRecurrence = "recurring"
	GoalRecurrenceOneOff    GoalRecurrence = "one_off"
)

// GoalStatus is the lifecycle state of a goal.
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusPaused    GoalStatus = "paused"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusFailed    GoalStatus = "failed"
)

// Goal is a user's goal definition (the structured primitive) stored under
// users/{uid}/goals. The owning uid is carried by the path, not a field.
type Goal struct {
	GoalID           string         `firestore:"goalId" json:"goalId"`
	Type             GoalType       `firestore:"type" json:"type"`
	Name             string         `firestore:"name" json:"name"`
	TargetValueMinor int64          `firestore:"targetValueMinor" json:"targetValueMinor"` // integer minor units (e.g. cents)
	Currency         string         `firestore:"currency" json:"currency"`
	TimeWindow       GoalTimeWindow `firestore:"timeWindow" json:"timeWindow"`
	// EndDate (YYYY-MM-DD) bounds a fixed window or a one-off goal. Empty for
	// recurring weekly/monthly goals.
	EndDate         string              `firestore:"endDate,omitempty" json:"endDate,omitempty"`
	Recurrence      GoalRecurrence      `firestore:"recurrence" json:"recurrence"`
	Filters         GoalFilters         `firestore:"filters" json:"filters"`
	AlertThresholds GoalAlertThresholds `firestore:"alertThresholds" json:"alertThresholds"`
	Status          GoalStatus          `firestore:"status" json:"status"`
	// BaselineValueMinor is computed at creation for types measured against a
	// prior period (Reduction, Emergency Fund), in integer minor units. Unused
	// for Spending Limit.
	BaselineValueMinor *int64 `firestore:"baselineValueMinor,omitempty" json:"baselineValueMinor,omitempty"`
	// ReductionPercent is set only for reduction goals: how much less than the
	// baseline period the user aims to spend (e.g. 10 → 10% less). The concrete
	// TargetValueMinor is frozen from the baseline at creation and does NOT move
	// as a recurring goal rolls over — every period is measured against that same
	// frozen target. To retarget against a newer period, update or recreate the
	// goal.
	ReductionPercent *float64 `firestore:"reductionPercent,omitempty" json:"reductionPercent,omitempty"`
	// ConversationID links the goal to the chat session that created it, for the
	// "view original conversation" affordance.
	ConversationID string    `firestore:"conversationId,omitempty" json:"conversationId,omitempty"`
	CreatedAt      time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time `firestore:"updatedAt" json:"updatedAt"`
}

// GoalFilters scopes which transactions count toward a goal. All fields are
// optional; an empty filter set counts all spending.
type GoalFilters struct {
	PFCPrimary string `firestore:"pfcPrimary,omitempty" json:"pfcPrimary,omitempty"`
	Merchant   string `firestore:"merchant,omitempty" json:"merchant,omitempty"`
	AccountID  string `firestore:"accountId,omitempty" json:"accountId,omitempty"`
}

// GoalAlertThresholds configures when the daily evaluator raises a notification.
// A nil trigger is disabled. Point-event triggers (e.g. a single large
// transaction) belong to the alert subsystem, not goals — a goal tracks
// aggregate progress toward a target.
type GoalAlertThresholds struct {
	// ProgressPercent fires when progress reaches this percentage of target
	// (e.g. 80 → notify at 80% of budget used).
	ProgressPercent *float64 `firestore:"progressPercent,omitempty" json:"progressPercent,omitempty"`
}

// ResolveWindow returns the [start, end] calendar period the goal is measured
// over — both at date granularity (midnight UTC), end inclusive. Recurring
// goals track the live period containing now; one-off goals stay pinned to the
// period they were created in. A fixed window always runs from creation to its
// EndDate.
//
// The window is computed in UTC regardless of now's location. Callers cap the
// spend query at min(now, end) and derive pace from where now sits between
// start and end.
func (g *Goal) ResolveWindow(now time.Time) (start, end time.Time, err error) {
	// Recurring goals track the live calendar period; one-offs stay anchored to
	// the period they were created in.
	anchor := g.CreatedAt
	if g.Recurrence == GoalRecurrenceRecurring {
		anchor = now
	}

	switch g.TimeWindow {
	case GoalWindowMonthly:
		start = helpers.FirstOfMonth(anchor)
		return start, start.AddDate(0, 1, -1), nil // last day of the month
	case GoalWindowWeekly:
		start = helpers.MondayOf(anchor)
		return start, start.AddDate(0, 0, 6), nil // Sunday
	case GoalWindowFixed:
		end, err = helpers.ParseDate(g.EndDate)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("goal %s: invalid endDate %q: %w", g.GoalID, g.EndDate, err)
		}
		return helpers.DateOf(g.CreatedAt), end, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("goal %s: cannot resolve window for timeWindow %q", g.GoalID, g.TimeWindow)
	}
}
