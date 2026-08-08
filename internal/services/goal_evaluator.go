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
}

type goalEvaluatorAnalytics interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
}

// goalEvaluatorService writes a daily progress snapshot for every active goal.
// It's the deterministic half of the goals feature — measurement only; the
// notification triggers and AI insight are layered on separately.
type goalEvaluatorService struct {
	users     goalEvaluatorUserStore
	goals     goalEvaluatorGoalStore
	snapshots goalEvaluatorSnapshotStore
	analytics goalEvaluatorAnalytics
	clockNow  func() time.Time
}

func NewGoalEvaluatorService(
	users goalEvaluatorUserStore,
	goals goalEvaluatorGoalStore,
	snapshots goalEvaluatorSnapshotStore,
	analytics goalEvaluatorAnalytics,
) *goalEvaluatorService {
	return &goalEvaluatorService{
		users:     users,
		goals:     goals,
		snapshots: snapshots,
		analytics: analytics,
		clockNow:  time.Now,
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
		snap, err := s.evaluateGoal(ctx, uid, goal, now)
		if err != nil {
			log.Error("failed to evaluate goal", "uid", uid, "goalId", goal.GoalID, "error", err)
			continue
		}
		if err := s.snapshots.Create(ctx, uid, snap); err != nil {
			log.Error("failed to write goal snapshot", "uid", uid, "goalId", goal.GoalID, "error", err)
		}
	}

	return nil
}

func (s *goalEvaluatorService) evaluateGoal(ctx context.Context, uid string, goal *models.Goal, now time.Time) (*models.GoalSnapshot, error) {
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
		return nil, fmt.Errorf("spend total: %w", err)
	}

	current := result.Total
	elapsed := goalElapsedFraction(start, queryTo, end)

	return &models.GoalSnapshot{
		SnapshotID:      uuid.NewString(),
		GoalID:          goal.GoalID,
		CreatedAt:       now,
		CurrentValue:    current,
		TargetValue:     goal.TargetValue,
		PercentComplete: goalPercentComplete(current, goal.TargetValue),
		// Pace-based: on track when spending hasn't outrun its share of the
		// window elapsed so far.
		IsOnTrack: current <= goal.TargetValue*elapsed,
	}, nil
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
