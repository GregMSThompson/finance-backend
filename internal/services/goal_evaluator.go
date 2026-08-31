package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type goalEvaluatorUserStore interface {
	List(ctx context.Context) ([]*models.User, error)
}

type goalEvaluatorGoalStore interface {
	List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error)
	Update(ctx context.Context, uid string, g *models.Goal) error
}

type goalEvaluatorSnapshotStore interface {
	Create(ctx context.Context, uid string, snap *models.GoalSnapshot) error
	ListForGoalSince(ctx context.Context, uid, goalID string, since time.Time) ([]*models.GoalSnapshot, error)
}

// goalStrategyAnalytics is the spend measurement a strategy needs, satisfied by
// the analytics service and shared by the create and evaluate paths.
type goalStrategyAnalytics interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
}

// goalStrategy encapsulates the type-specific parts of a goal: capturing any
// baseline at creation, measuring current progress in a window, resolving the
// effective target, and scoring a measurement. The evaluator owns the shared
// scaffolding (windowing, snapshots, notification dedup, terminal detection);
// a strategy supplies only what differs by type. The create path calls only
// Initialize (baseline capture); the evaluator drives Measure/Target/Score.
type goalStrategy interface {
	Initialize(ctx context.Context, uid string, g *models.Goal) error
	Measure(ctx context.Context, uid string, g *models.Goal, w goalWindow) (int64, error)
	Target(g *models.Goal) int64
	Score(g *models.Goal, w goalWindow, current int64) goalProgress
}

// goalEvaluatorService writes a daily progress snapshot for every active goal.
// It's the deterministic half of the goals feature — measurement only; the
// notification triggers and AI insight are layered on separately.
type goalEvaluatorService struct {
	users         goalEvaluatorUserStore
	goals         goalEvaluatorGoalStore
	snapshots     goalEvaluatorSnapshotStore
	strategies    map[models.GoalType]goalStrategy
	notifications evaluatorNotificationStore
	tasks         evaluatorTasksClient
}

// goalEvaluation is the outcome of evaluating one goal on one run: the snapshot
// to persist, an optional notification to dispatch, and an optional terminal
// status change (completed/failed) to apply.
type goalEvaluation struct {
	snapshot     *models.GoalSnapshot
	notification *models.Notification
	newStatus    models.GoalStatus
}

// goalWindow is the resolved measurement period for a single evaluation.
// queryTo is the earlier of today and end — a fixed window can close before
// today — and caps both the spend query and the pace clock.
type goalWindow struct {
	start   time.Time
	end     time.Time
	queryTo time.Time
}

// goalProgress is the scored outcome of a measurement. succeeded reports
// whether the goal would be met if the window closed now; it is consulted only
// on a one-off's terminal evaluation.
type goalProgress struct {
	percent   float64
	isOnTrack bool
	succeeded bool
}

// spendingLimitStrategy measures accumulated spend in the window against a fixed
// ceiling: lower is better, on track while spend stays within its share of the
// window elapsed, met while spend stays at or under target.
type spendingLimitStrategy struct {
	analytics goalStrategyAnalytics
}

// reductionStrategy is a spending limit whose ceiling is derived, at creation,
// from a comparable prior period: target = baseline × (1 − percent). Once frozen
// it behaves exactly like a spending limit — including across recurring periods,
// which reuse the same frozen target rather than re-measuring the baseline.
type reductionStrategy struct {
	spendingLimitStrategy
}

func NewGoalEvaluatorService(
	users goalEvaluatorUserStore,
	goals goalEvaluatorGoalStore,
	snapshots goalEvaluatorSnapshotStore,
	analytics goalStrategyAnalytics,
	notifications evaluatorNotificationStore,
	tasks evaluatorTasksClient,
) *goalEvaluatorService {
	return &goalEvaluatorService{
		users:         users,
		goals:         goals,
		snapshots:     snapshots,
		strategies:    newGoalStrategies(analytics),
		notifications: notifications,
		tasks:         tasks,
	}
}

// Run evaluates every user's active goals against a single run clock. A failure
// for one user (or one goal) is logged and skipped so a single bad record can't
// sink the whole batch.
func (s *goalEvaluatorService) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	users, err := s.users.List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	log.Info("starting goal evaluation", "userCount", len(users))

	now := clock.Now(ctx)
	for _, user := range users {
		if err := s.evaluateUser(ctx, user.UID, now); err != nil {
			log.Error("failed to evaluate user goals", "uid", user.UID, "error", err)
		}
	}

	return nil
}

func (s *goalEvaluatorService) evaluateUser(ctx context.Context, uid string, now time.Time) error {
	log := logger.FromContext(ctx)

	goals, err := s.goals.List(ctx, uid, models.GoalStatusActive)
	if err != nil {
		return fmt.Errorf("list goals: %w", err)
	}

	for _, goal := range goals {
		eval, err := s.evaluateGoal(ctx, uid, goal, now)
		if err != nil {
			log.Error("failed to evaluate goal", "uid", uid, "goalId", goal.GoalID, "error", err)
			continue
		}
		// Persist the snapshot before dispatching: its PercentComplete is what
		// arms the period-scoped dedup, so a delivered notification without a
		// recorded snapshot could double-fire next run.
		if err := s.snapshots.Create(ctx, uid, eval.snapshot); err != nil {
			log.Error("failed to write goal snapshot", "uid", uid, "goalId", goal.GoalID, "error", err)
			continue
		}
		// A terminal transition flips status before dispatch: that flip removes
		// the goal from the active set, so it's what makes the terminal
		// notification fire exactly once. If it fails, skip dispatch and let the
		// next run retry rather than risk a duplicate.
		if eval.newStatus != "" {
			goal.Status = eval.newStatus
			if err := s.goals.Update(ctx, uid, goal); err != nil {
				log.Error("failed to update goal status", "uid", uid, "goalId", goal.GoalID, "status", eval.newStatus, "error", err)
				continue
			}
		}
		if eval.notification != nil {
			if err := s.dispatch(ctx, uid, eval.notification); err != nil {
				log.Error("failed to dispatch goal notification", "uid", uid, "goalId", goal.GoalID, "error", err)
			}
		}
	}

	return nil
}

// dispatch records the notification and enqueues its delivery through the shared
// notification pipeline.
func (s *goalEvaluatorService) dispatch(ctx context.Context, uid string, n *models.Notification) error {
	if err := s.notifications.Create(ctx, uid, n); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	if err := s.tasks.EnqueueNotificationDelivery(ctx, dto.DeliverNotificationRequest{
		NotificationID: n.NotificationID,
		UserID:         uid,
		Delivery:       n.Delivery,
	}); err != nil {
		return fmt.Errorf("enqueue notification delivery: %w", err)
	}
	return nil
}

func (s *goalEvaluatorService) evaluateGoal(ctx context.Context, uid string, goal *models.Goal, now time.Time) (*goalEvaluation, error) {
	start, end, err := goal.ResolveWindow(now)
	if err != nil {
		return nil, err
	}

	// Cap the spend query and the pace clock at the earlier of today and the
	// window end — a fixed window can close before today.
	queryTo := helpers.DateOf(now)
	if queryTo.After(end) {
		queryTo = end
	}
	w := goalWindow{start: start, end: end, queryTo: queryTo}

	strat, err := strategyFor(s.strategies, goal.Type)
	if err != nil {
		return nil, err
	}

	current, err := strat.Measure(ctx, uid, goal, w)
	if err != nil {
		return nil, err
	}
	prog := strat.Score(goal, w, current)

	snap := &models.GoalSnapshot{
		SnapshotID:        uuid.NewString(),
		GoalID:            goal.GoalID,
		CreatedAt:         now,
		CurrentValueMinor: current,
		TargetValueMinor:  strat.Target(goal),
		Currency:          goal.Currency,
		PercentComplete:   prog.percent,
		IsOnTrack:         prog.isOnTrack,
	}

	eval := &goalEvaluation{snapshot: snap}

	// Terminal transition: a one-off whose window has closed resolves for good.
	// Recurring goals reset each period and never terminate here. The final
	// value is stable (queryTo is capped at the window end), so the outcome is
	// the same regardless of which run past the end date observes it.
	if goal.Recurrence == models.GoalRecurrenceOneOff && helpers.DateOf(now).After(end) {
		eval.newStatus = models.GoalStatusFailed
		if prog.succeeded {
			eval.newStatus = models.GoalStatusCompleted
		}
		body, err := goalTerminalText(goal, snap, eval.newStatus)
		if err != nil {
			return nil, err
		}
		snap.AIInsight = body
		snap.NotificationSent = true
		eval.notification = buildGoalNotification(goal, body, now)
		return eval, nil
	}

	// Otherwise, the progress-threshold notification (terminal supersedes it).
	notification, err := s.maybeNotify(ctx, uid, goal, snap, start)
	if err != nil {
		return nil, err
	}
	eval.notification = notification
	return eval, nil
}

// maybeNotify fires a progress-threshold notification on a state transition —
// crossing over the threshold or dropping back under it — but stays silent while
// the state is unchanged. It records the insight and NotificationSent on snap
// and returns the Notification to dispatch, or nil when nothing is due.
//
// The previous state is the most recent snapshot this period; a fresh period has
// none, so it baselines to "under" (spend has reset) and a period rollover is
// silent. Comparing to the previous snapshot rather than a fixed rule keeps this
// independent of how often the evaluator runs.
func (s *goalEvaluatorService) maybeNotify(ctx context.Context, uid string, goal *models.Goal, snap *models.GoalSnapshot, periodStart time.Time) (*models.Notification, error) {
	threshold := goal.AlertThresholds.ProgressPercent
	if threshold == nil {
		return nil, nil
	}

	prior, err := s.snapshots.ListForGoalSince(ctx, uid, goal.GoalID, periodStart)
	if err != nil {
		return nil, fmt.Errorf("list snapshots since period start: %w", err)
	}

	over := snap.PercentComplete >= *threshold
	// prior is ordered most-recent first; the current snapshot isn't written yet,
	// so prior[0] is the previous state this period. No prior ⇒ baseline "under".
	wasOver := len(prior) > 0 && prior[0].PercentComplete >= *threshold
	if over == wasOver {
		return nil, nil
	}

	insight, err := goalProgressText(goal, snap, over)
	if err != nil {
		return nil, err
	}
	snap.AIInsight = insight
	snap.NotificationSent = true

	return buildGoalNotification(goal, insight, snap.CreatedAt), nil
}

// buildGoalNotification assembles the push notification shared by the progress
// and terminal triggers: a goal-sourced message deep-linking to the goal.
func buildGoalNotification(goal *models.Goal, body string, now time.Time) *models.Notification {
	return &models.Notification{
		NotificationID: uuid.NewString(),
		Source:         models.NotificationSourceGoal,
		SourceID:       goal.GoalID,
		Title:          goal.Name,
		Body:           body,
		Data: map[string]string{
			"source":   string(models.NotificationSourceGoal),
			"sourceId": goal.GoalID,
		},
		Delivery:  models.DeliveryPush,
		CreatedAt: now,
	}
}

// goalTerminalText renders the body for a one-off goal's completed/failed
// outcome. Deterministic, same as the progress placeholder — see the
// TODO(goals) note on goalInsightText for the richer AI insight to come.
func goalTerminalText(goal *models.Goal, snap *models.GoalSnapshot, status models.GoalStatus) (string, error) {
	current, err := helpers.FormatCurrency(snap.CurrentValueMinor, goal.Currency)
	if err != nil {
		return "", err
	}
	target, err := helpers.FormatCurrency(snap.TargetValueMinor, goal.Currency)
	if err != nil {
		return "", err
	}
	if status == models.GoalStatusCompleted {
		return fmt.Sprintf("You completed your %s goal — spent %s, within your %s limit.",
			goal.Name, current, target), nil
	}
	return fmt.Sprintf("Your %s goal ended over budget — spent %s of your %s limit.",
		goal.Name, current, target), nil
}

// goalProgressText renders the notification body for a threshold transition —
// over is true when spending has just crossed the threshold, false when it has
// dropped back under.
//
// TODO(goals): this is a deterministic placeholder that only restates the
// numbers. The real feature is a personalized AI insight that pulls richer
// context — spend breakdown by merchant/category within the goal's scope,
// comparison to prior periods, and an end-of-period projection — so it can say
// something specific and actionable. That deserves its own piece (deciding
// which analytics to pull, prompt design, cost/latency); when we build it,
// reintroduce a genai dependency on the evaluator and swap this call out.
func goalProgressText(goal *models.Goal, snap *models.GoalSnapshot, over bool) (string, error) {
	current, err := helpers.FormatCurrency(snap.CurrentValueMinor, goal.Currency)
	if err != nil {
		return "", err
	}
	target, err := helpers.FormatCurrency(snap.TargetValueMinor, goal.Currency)
	if err != nil {
		return "", err
	}
	remaining, err := helpers.FormatCurrency(snap.TargetValueMinor-snap.CurrentValueMinor, goal.Currency)
	if err != nil {
		return "", err
	}
	if over {
		return fmt.Sprintf("You've used %s of your %s budget — %s of %s, %s remaining.",
			helpers.FormatPercent(snap.PercentComplete),
			goal.Name, current, target, remaining), nil
	}
	return fmt.Sprintf("Your %s spending is back down to %s — %s of %s, %s remaining.",
		goal.Name,
		helpers.FormatPercent(snap.PercentComplete),
		current, target, remaining), nil
}

// goalElapsedFraction returns how far through the window queryTo sits, in whole
// inclusive days, clamped to [0,1]. Day 1 of a 30-day window is 1/30, the last
// day is 1. Single-day windows are 1, which avoids a divide-by-zero.
func goalElapsedFraction(start, queryTo, end time.Time) float64 {
	totalDays := daysInclusive(start, end)
	if totalDays <= 1 {
		return 1
	}
	f := float64(daysInclusive(start, queryTo)) / float64(totalDays)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// daysInclusive counts calendar days from..to inclusive. Both are expected at
// date (midnight UTC) granularity, so the difference is an exact day multiple.
func daysInclusive(from, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
}

func goalPercentComplete(current, target int64) float64 {
	if target <= 0 {
		return 0
	}
	return float64(current) / float64(target) * 100
}

// --- Goal strategies -------------------------------------------------------

func newGoalStrategies(analytics goalStrategyAnalytics) map[models.GoalType]goalStrategy {
	sl := spendingLimitStrategy{analytics: analytics}
	return map[models.GoalType]goalStrategy{
		models.GoalTypeSpendingLimit: sl,
		models.GoalTypeReduction:     reductionStrategy{spendingLimitStrategy: sl},
	}
}

// strategyFor resolves a goal type to its strategy, rejecting unsupported types
// the same way validation does.
func strategyFor(strategies map[models.GoalType]goalStrategy, t models.GoalType) (goalStrategy, error) {
	strat, ok := strategies[t]
	if !ok {
		return nil, errs.NewValidationError(fmt.Sprintf("unsupported goal type: %s", t))
	}
	return strat, nil
}

func (s spendingLimitStrategy) Initialize(ctx context.Context, uid string, g *models.Goal) error {
	return nil // a literal target needs no baseline
}

func (s spendingLimitStrategy) Measure(ctx context.Context, uid string, g *models.Goal, w goalWindow) (int64, error) {
	args := dto.AnalyticsSpendTotalArgs{
		Pending:  helpers.Ptr(false),
		DateFrom: helpers.Ptr(helpers.FormatDate(w.start)),
		DateTo:   helpers.Ptr(helpers.FormatDate(w.queryTo)),
	}
	applyGoalFilters(&args, g.Filters)
	result, err := s.analytics.GetSpendTotal(ctx, uid, args)
	if err != nil {
		return 0, fmt.Errorf("spend total: %w", err)
	}
	return result.TotalMinor, nil
}

func (s spendingLimitStrategy) Target(g *models.Goal) int64 {
	return g.TargetValueMinor
}

func (s spendingLimitStrategy) Score(g *models.Goal, w goalWindow, current int64) goalProgress {
	target := g.TargetValueMinor
	elapsed := goalElapsedFraction(w.start, w.queryTo, w.end)
	return goalProgress{
		percent:   goalPercentComplete(current, target),
		isOnTrack: float64(current) <= float64(target)*elapsed,
		succeeded: current <= target,
	}
}

func (s reductionStrategy) Initialize(ctx context.Context, uid string, g *models.Goal) error {
	if g.ReductionPercent == nil {
		return errs.NewValidationError("reductionPercent is required for a reduction goal")
	}
	start, end, err := priorComparableWindow(g)
	if err != nil {
		return err
	}
	args := dto.AnalyticsSpendTotalArgs{
		Pending:  helpers.Ptr(false),
		DateFrom: helpers.Ptr(helpers.FormatDate(start)),
		DateTo:   helpers.Ptr(helpers.FormatDate(end)),
	}
	applyGoalFilters(&args, g.Filters)
	result, err := s.analytics.GetSpendTotal(ctx, uid, args)
	if err != nil {
		return fmt.Errorf("reduction baseline spend total: %w", err)
	}
	baseline := result.TotalMinor
	g.BaselineValueMinor = &baseline
	g.TargetValueMinor = int64(math.Round(float64(baseline) * (100 - *g.ReductionPercent) / 100))
	return nil
}

// priorComparableWindow returns the last complete period of the goal's shape
// immediately before the period containing its creation — the baseline a
// reduction goal measures against. Monthly → previous calendar month, weekly →
// previous week, fixed → the equal-length window ending the day before.
func priorComparableWindow(g *models.Goal) (start, end time.Time, err error) {
	curStart, curEnd, err := g.ResolveWindow(g.CreatedAt)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	switch g.TimeWindow {
	case models.GoalWindowMonthly:
		lastDayPrev := curStart.AddDate(0, 0, -1)
		return helpers.FirstOfMonth(lastDayPrev), lastDayPrev, nil
	case models.GoalWindowWeekly:
		return curStart.AddDate(0, 0, -7), curStart.AddDate(0, 0, -1), nil
	case models.GoalWindowFixed:
		lengthDays := daysInclusive(curStart, curEnd)
		end = curStart.AddDate(0, 0, -1)
		return end.AddDate(0, 0, -(lengthDays - 1)), end, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("goal %s: cannot resolve prior window for timeWindow %q", g.GoalID, g.TimeWindow)
	}
}

// applyGoalFilters maps a goal's filter set onto a spend-total query. OptString
// leaves an unset filter nil so it doesn't constrain the query.
func applyGoalFilters(args *dto.AnalyticsSpendTotalArgs, f models.GoalFilters) {
	args.PFCPrimary = helpers.OptString(f.PFCPrimary)
	args.Merchant = helpers.OptString(f.Merchant)
	args.AccountID = helpers.OptString(f.AccountID)
}
