package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type goalEvaluatorUserStore interface {
	List(ctx context.Context) ([]*models.User, error)
}

type goalEvaluatorGoalStore interface {
	List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error)
}

type goalEvaluatorSnapshotStore interface {
	Create(ctx context.Context, uid string, snap *models.GoalSnapshot) error
	ListForGoalSince(ctx context.Context, uid, goalID string, since time.Time) ([]*models.GoalSnapshot, error)
}

type goalEvaluatorAnalytics interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
}

// goalEvaluatorService writes a daily progress snapshot for every active goal.
// It's the deterministic half of the goals feature — measurement only; the
// notification triggers and AI insight are layered on separately.
type goalEvaluatorService struct {
	users         goalEvaluatorUserStore
	goals         goalEvaluatorGoalStore
	snapshots     goalEvaluatorSnapshotStore
	analytics     goalEvaluatorAnalytics
	notifications evaluatorNotificationStore
	tasks         evaluatorTasksClient
	clockNow      func() time.Time
}

func NewGoalEvaluatorService(
	users goalEvaluatorUserStore,
	goals goalEvaluatorGoalStore,
	snapshots goalEvaluatorSnapshotStore,
	analytics goalEvaluatorAnalytics,
	notifications evaluatorNotificationStore,
	tasks evaluatorTasksClient,
) *goalEvaluatorService {
	return &goalEvaluatorService{
		users:         users,
		goals:         goals,
		snapshots:     snapshots,
		analytics:     analytics,
		notifications: notifications,
		tasks:         tasks,
		clockNow:      time.Now,
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

	now := s.clockNow()
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
		snap, notification, err := s.evaluateGoal(ctx, uid, goal, now)
		if err != nil {
			log.Error("failed to evaluate goal", "uid", uid, "goalId", goal.GoalID, "error", err)
			continue
		}
		// Persist the snapshot before dispatching: its PercentComplete is what
		// arms the period-scoped dedup, so a delivered notification without a
		// recorded snapshot could double-fire next run.
		if err := s.snapshots.Create(ctx, uid, snap); err != nil {
			log.Error("failed to write goal snapshot", "uid", uid, "goalId", goal.GoalID, "error", err)
			continue
		}
		if notification != nil {
			if err := s.dispatch(ctx, uid, notification); err != nil {
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

func (s *goalEvaluatorService) evaluateGoal(ctx context.Context, uid string, goal *models.Goal, now time.Time) (*models.GoalSnapshot, *models.Notification, error) {
	start, end, err := goal.ResolveWindow(now)
	if err != nil {
		return nil, nil, err
	}

	// Cap the spend query and the pace clock at the earlier of today and the
	// window end — a fixed window can close before today.
	queryTo := helpers.DateOf(now)
	if queryTo.After(end) {
		queryTo = end
	}

	args := dto.AnalyticsSpendTotalArgs{
		Pending:  helpers.Ptr(false),
		DateFrom: helpers.Ptr(helpers.FormatDate(start)),
		DateTo:   helpers.Ptr(helpers.FormatDate(queryTo)),
	}
	if goal.Filters.PFCPrimary != "" {
		args.PFCPrimary = helpers.Ptr(goal.Filters.PFCPrimary)
	}
	if goal.Filters.Merchant != "" {
		args.Merchant = helpers.Ptr(goal.Filters.Merchant)
	}
	if goal.Filters.AccountID != "" {
		args.AccountID = helpers.Ptr(goal.Filters.AccountID)
	}

	result, err := s.analytics.GetSpendTotal(ctx, uid, args)
	if err != nil {
		return nil, nil, fmt.Errorf("spend total: %w", err)
	}

	current := result.Total
	elapsed := goalElapsedFraction(start, queryTo, end)
	percent := goalPercentComplete(current, goal.TargetValue)

	snap := &models.GoalSnapshot{
		SnapshotID:      uuid.NewString(),
		GoalID:          goal.GoalID,
		CreatedAt:       now,
		CurrentValue:    current,
		TargetValue:     goal.TargetValue,
		PercentComplete: percent,
		// Pace-based: on track when spending hasn't outrun its share of the
		// window elapsed so far.
		IsOnTrack: current <= goal.TargetValue*elapsed,
	}

	notification, err := s.maybeNotify(ctx, uid, goal, snap, result.Currency, start)
	if err != nil {
		return nil, nil, err
	}
	return snap, notification, nil
}

// maybeNotify fires the progress-threshold notification when it's newly reached
// this period. It records the insight and NotificationSent on snap and returns
// the Notification to dispatch, or nil when nothing is due.
func (s *goalEvaluatorService) maybeNotify(ctx context.Context, uid string, goal *models.Goal, snap *models.GoalSnapshot, currency string, periodStart time.Time) (*models.Notification, error) {
	threshold := goal.AlertThresholds.ProgressPercent
	if threshold == nil || snap.PercentComplete < *threshold {
		return nil, nil
	}

	// Period-scoped dedup: notify only if no snapshot earlier this period already
	// reached the threshold. This makes the "one alert per period" guarantee
	// independent of how often the evaluator runs.
	prior, err := s.snapshots.ListForGoalSince(ctx, uid, goal.GoalID, periodStart)
	if err != nil {
		return nil, fmt.Errorf("list snapshots since period start: %w", err)
	}
	for _, p := range prior {
		if p.PercentComplete >= *threshold {
			return nil, nil
		}
	}

	insight := goalInsightText(goal, snap, currency)
	snap.AIInsight = insight
	snap.NotificationSent = true

	return &models.Notification{
		NotificationID: uuid.NewString(),
		Source:         models.NotificationSourceGoal,
		SourceID:       goal.GoalID,
		Title:          goal.Name,
		Body:           insight,
		Data: map[string]string{
			"source":   string(models.NotificationSourceGoal),
			"sourceId": goal.GoalID,
		},
		Delivery:  models.DeliveryPush,
		CreatedAt: snap.CreatedAt,
	}, nil
}

// goalInsightText renders the notification body.
//
// TODO(goals): this is a deterministic placeholder that only restates the
// numbers. The real feature is a personalized AI insight that pulls richer
// context — spend breakdown by merchant/category within the goal's scope,
// comparison to prior periods, and an end-of-period projection — so it can say
// something specific and actionable. That deserves its own piece (deciding
// which analytics to pull, prompt design, cost/latency); when we build it,
// reintroduce a genai dependency on the evaluator and swap this call out.
func goalInsightText(goal *models.Goal, snap *models.GoalSnapshot, currency string) string {
	return fmt.Sprintf("You've used %s of your %s budget — %s of %s, %s remaining.",
		helpers.FormatPercent(snap.PercentComplete),
		goal.Name,
		helpers.FormatCurrency(snap.CurrentValue, currency),
		helpers.FormatCurrency(snap.TargetValue, currency),
		helpers.FormatCurrency(snap.TargetValue-snap.CurrentValue, currency),
	)
}

func fallbackInsight(goal *models.Goal, snap *models.GoalSnapshot, currency string) string {
	return fmt.Sprintf("You've used %s of your %s budget (%s of %s).",
		helpers.FormatPercent(snap.PercentComplete),
		goal.Name,
		helpers.FormatCurrency(snap.CurrentValue, currency),
		helpers.FormatCurrency(snap.TargetValue, currency),
	)
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

func goalPercentComplete(current, target float64) float64 {
	if target <= 0 {
		return 0
	}
	return current / target * 100
}
