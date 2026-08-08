package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type fakeEvalUserStore struct {
	users []*models.User
	err   error
}

func (f *fakeEvalUserStore) List(_ context.Context) ([]*models.User, error) {
	return f.users, f.err
}

type fakeEvalAnalytics struct {
	calls  []dto.AnalyticsSpendTotalArgs
	fn     func(dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
	result dto.AnalyticsSpendTotalResult
	err    error
}

func (f *fakeEvalAnalytics) GetSpendTotal(_ context.Context, _ string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error) {
	f.calls = append(f.calls, args)
	if f.fn != nil {
		return f.fn(args)
	}
	return f.result, f.err
}

// clock is a mid-August run time used across the evaluator tests: the 15th of a
// 31-day month, so a monthly window spans [2026-08-01, 2026-08-31].
var evalClock = time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)

func newEvaluator(users *fakeEvalUserStore, goals *fakeGoalStore, snaps *fakeGoalSnapshotStore, analytics *fakeEvalAnalytics) *goalEvaluatorService {
	svc := NewGoalEvaluatorService(users, goals, snaps, analytics)
	svc.clockNow = func() time.Time { return evalClock }
	return svc
}

func monthlyGoal(id string, target float64) *models.Goal {
	return &models.Goal{
		GoalID:      id,
		Type:        models.GoalTypeSpendingLimit,
		TargetValue: target,
		TimeWindow:  models.GoalWindowMonthly,
		Recurrence:  models.GoalRecurrenceRecurring,
		Status:      models.GoalStatusActive,
	}
}

func TestGoalEvaluator_SnapshotPerActiveGoal(t *testing.T) {
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{
		"g1": monthlyGoal("g1", 300),
		"g2": monthlyGoal("g2", 100),
	}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 120, Currency: "USD"}}

	if err := newEvaluator(users, goals, snaps, analytics).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(goals.lastListStatuses) != 1 || goals.lastListStatuses[0] != models.GoalStatusActive {
		t.Fatalf("expected goals listed as [active], got %v", goals.lastListStatuses)
	}
	if len(snaps.created) != 2 {
		t.Fatalf("expected a snapshot per active goal (2), got %d", len(snaps.created))
	}

	byGoal := map[string]*models.GoalSnapshot{}
	for _, s := range snaps.created {
		byGoal[s.GoalID] = s
	}
	if s := byGoal["g1"]; s == nil || s.CurrentValue != 120 || s.TargetValue != 300 || s.PercentComplete != 40 {
		t.Fatalf("g1 snapshot wrong: %+v", s)
	}
	if s := byGoal["g2"]; s == nil || s.PercentComplete != 120 {
		t.Fatalf("g2 snapshot wrong: %+v", s)
	}
	if byGoal["g1"].CreatedAt != evalClock {
		t.Fatalf("snapshot createdAt should be the run clock, got %s", byGoal["g1"].CreatedAt)
	}
}

func TestGoalEvaluator_MapsFiltersAndWindow(t *testing.T) {
	g := monthlyGoal("g1", 300)
	g.Filters = models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK", Merchant: "cafe", AccountID: "acc1"}

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 10}}

	if err := newEvaluator(users, goals, &fakeGoalSnapshotStore{}, analytics).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(analytics.calls) != 1 {
		t.Fatalf("expected 1 spend-total call, got %d", len(analytics.calls))
	}
	args := analytics.calls[0]
	if helpers.Value(args.Pending) != false {
		t.Fatalf("expected pending=false")
	}
	if helpers.Value(args.PFCPrimary) != "FOOD_AND_DRINK" || helpers.Value(args.Merchant) != "cafe" || helpers.Value(args.AccountID) != "acc1" {
		t.Fatalf("filters not mapped: %+v", args)
	}
	if helpers.Value(args.DateFrom) != "2026-08-01" || helpers.Value(args.DateTo) != "2026-08-15" {
		t.Fatalf("window mismatch: from=%q to=%q", helpers.Value(args.DateFrom), helpers.Value(args.DateTo))
	}
}

func TestGoalEvaluator_PaceOnTrack(t *testing.T) {
	// On 2026-08-15, 15 of 31 days elapsed ≈ 48.4%; target 300 → pace ≈ 145.
	cases := []struct {
		name    string
		spent   float64
		onTrack bool
	}{
		{"under pace", 100, true},
		{"over pace", 200, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
			goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": monthlyGoal("g1", 300)}}
			snaps := &fakeGoalSnapshotStore{}
			analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: tc.spent}}

			if err := newEvaluator(users, goals, snaps, analytics).Run(helpers.TestCtx()); err != nil {
				t.Fatalf("Run error: %v", err)
			}
			if len(snaps.created) != 1 {
				t.Fatalf("expected 1 snapshot, got %d", len(snaps.created))
			}
			if snaps.created[0].IsOnTrack != tc.onTrack {
				t.Fatalf("spent %v: isOnTrack=%v, want %v", tc.spent, snaps.created[0].IsOnTrack, tc.onTrack)
			}
		})
	}
}

func TestGoalEvaluator_PerGoalErrorIsolation(t *testing.T) {
	// g1 has a fixed window with an unparseable EndDate, so ResolveWindow errors;
	// g2 is a normal monthly goal. The bad goal must not stop the good one.
	bad := &models.Goal{
		GoalID:      "g1",
		Type:        models.GoalTypeSpendingLimit,
		TargetValue: 100,
		TimeWindow:  models.GoalWindowFixed,
		Recurrence:  models.GoalRecurrenceOneOff,
		EndDate:     "not-a-date",
		Status:      models.GoalStatusActive,
	}
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": bad, "g2": monthlyGoal("g2", 100)}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 50}}

	if err := newEvaluator(users, goals, snaps, analytics).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run should not fail on a single bad goal: %v", err)
	}
	if len(snaps.created) != 1 || snaps.created[0].GoalID != "g2" {
		t.Fatalf("expected only g2 snapshotted, got %+v", snaps.created)
	}
}

func TestGoalEvaluator_UserListErrorPropagates(t *testing.T) {
	users := &fakeEvalUserStore{err: errors.New("firestore down")}
	svc := newEvaluator(users, &fakeGoalStore{goals: map[string]*models.Goal{}}, &fakeGoalSnapshotStore{}, &fakeEvalAnalytics{})

	if err := svc.Run(helpers.TestCtx()); err == nil {
		t.Fatal("expected Run to fail when listing users fails")
	}
}
