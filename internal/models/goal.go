package models

import (
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// GoalType identifies the kind of target a goal tracks.
type GoalType string

const (
	GoalTypeSpendingLimit        GoalType = "spending_limit"
	GoalTypeReduction            GoalType = "reduction"
	GoalTypeNetSavings           GoalType = "net_savings"
	GoalTypeIncomeTarget         GoalType = "income_target"
	GoalTypeSavingsTarget        GoalType = "savings_target"
	GoalTypeSavingsContributions GoalType = "savings_contributions"
	GoalTypePayDown              GoalType = "pay_down"
	GoalTypeEmergencyFund        GoalType = "emergency_fund"
	GoalTypeFrequencyLimit       GoalType = "frequency_limit"
)

// GoalTimeWindow is the period a goal is measured over.
type GoalTimeWindow string

const (
	GoalWindowWeekly       GoalTimeWindow = "weekly"
	GoalWindowMonthly      GoalTimeWindow = "monthly"
	GoalWindowFixed        GoalTimeWindow = "fixed" // bounded by EndDate
	GoalWindowUntilReached GoalTimeWindow = "until_reached"
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
	GoalID           string   `firestore:"goalId" json:"goalId"`
	Type             GoalType `firestore:"type" json:"type"`
	Name             string   `firestore:"name" json:"name"`
	TargetValueMinor int64    `firestore:"targetValueMinor" json:"targetValueMinor"` // integer minor units (e.g. cents)
	// TargetCount is the target for count-measured goals (frequency_limit): a plain
	// count of matching transactions, not money. It is the count-unit counterpart to
	// TargetValueMinor — validation guarantees exactly one of the two is set, so a
	// goal's target is always unambiguous. Kept separate (rather than reusing
	// TargetValueMinor) so a count never flows through the money-conversion layers
	// (the AI major/minor rewrite, the UI's currency formatting).
	TargetCount int64          `firestore:"targetCount,omitempty" json:"targetCount,omitempty"`
	Currency    string         `firestore:"currency" json:"currency"`
	TimeWindow  GoalTimeWindow `firestore:"timeWindow" json:"timeWindow"`
	// EndDate (YYYY-MM-DD) bounds a fixed window or a one-off goal. Empty for
	// recurring weekly/monthly goals.
	EndDate         string              `firestore:"endDate,omitempty" json:"endDate,omitempty"`
	Recurrence      GoalRecurrence      `firestore:"recurrence" json:"recurrence"`
	Filters         GoalFilters         `firestore:"filters" json:"filters"`
	AlertThresholds GoalAlertThresholds `firestore:"alertThresholds" json:"alertThresholds"`
	Status          GoalStatus          `firestore:"status" json:"status"`
	// BaselineValueMinor is captured at creation for types measured against a
	// starting point, in integer minor units: Reduction (prior-period spend) and
	// Savings Target (the account balance at creation, so progress measures new
	// saving since then). Unused for Spending Limit.
	BaselineValueMinor *int64 `firestore:"baselineValueMinor,omitempty" json:"baselineValueMinor,omitempty"`
	// ReductionPercent is set only for reduction goals: how much less than the
	// baseline period the user aims to spend (e.g. 10 → 10% less). The concrete
	// TargetValueMinor is frozen from the baseline at creation and does NOT move
	// as a recurring goal rolls over — every period is measured against that same
	// frozen target. To retarget against a newer period, update or recreate the
	// goal.
	ReductionPercent *float64 `firestore:"reductionPercent,omitempty" json:"reductionPercent,omitempty"`
	// MonthsOfExpenses is set only for emergency_fund goals: how many months of
	// expenses to save. The concrete TargetValueMinor is derived at creation from
	// the user's average monthly spend × this many months, then frozen — it does
	// not move as spending changes. To retarget, update or recreate the goal.
	MonthsOfExpenses *float64 `firestore:"monthsOfExpenses,omitempty" json:"monthsOfExpenses,omitempty"`
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
	case GoalWindowUntilReached:
		// No deadline: the window spans creation to today. Completion is driven by
		// reaching the target, not by the window closing, and pace isn't scored, so
		// the end is just "today" rather than a fixed date.
		return helpers.DateOf(g.CreatedAt), helpers.DateOf(now), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("goal %s: cannot resolve window for timeWindow %q", g.GoalID, g.TimeWindow)
	}
}
