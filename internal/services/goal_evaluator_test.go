package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
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

	incomeCalls  []dto.AnalyticsIncomeTotalArgs
	incomeResult dto.AnalyticsIncomeTotalResult
	incomeErr    error

	contribCalls  []dto.AnalyticsContributionsTotalArgs
	contribResult dto.AnalyticsContributionsTotalResult
	contribErr    error

	avgSpendCalls  int
	avgSpendResult dto.AnalyticsAverageMonthlySpendResult
	avgSpendErr    error
}

func (f *fakeEvalAnalytics) GetSpendTotal(_ context.Context, _ string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error) {
	f.calls = append(f.calls, args)
	if f.fn != nil {
		return f.fn(args)
	}
	return f.result, f.err
}

func (f *fakeEvalAnalytics) GetIncomeTotal(_ context.Context, _ string, args dto.AnalyticsIncomeTotalArgs) (dto.AnalyticsIncomeTotalResult, error) {
	f.incomeCalls = append(f.incomeCalls, args)
	return f.incomeResult, f.incomeErr
}

func (f *fakeEvalAnalytics) GetContributionsTotal(_ context.Context, _ string, args dto.AnalyticsContributionsTotalArgs) (dto.AnalyticsContributionsTotalResult, error) {
	f.contribCalls = append(f.contribCalls, args)
	return f.contribResult, f.contribErr
}

func (f *fakeEvalAnalytics) GetAverageMonthlySpend(_ context.Context, _ string, _ int) (dto.AnalyticsAverageMonthlySpendResult, error) {
	f.avgSpendCalls++
	return f.avgSpendResult, f.avgSpendErr
}

// fakeGoalAccounts stubs the balance dependency for goal tests. calls records the
// accountId filter each call received (nil for whole-finances) so a test can
// assert scoping.
type fakeGoalAccounts struct {
	balance int64
	err     error
	calls   []*string
}

func (f *fakeGoalAccounts) GetTotalBalance(_ context.Context, _ string, accountID *string) (int64, error) {
	f.calls = append(f.calls, accountID)
	return f.balance, f.err
}

// clock is a mid-August run time used across the evaluator tests: the 15th of a
// 31-day month, so a monthly window spans [2026-08-01, 2026-08-31].
var evalClock = time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)

func evalContext() context.Context {
	return clock.WithClock(helpers.TestCtx(), func() time.Time { return evalClock })
}

func newEvaluator(users *fakeEvalUserStore, goals *fakeGoalStore, snaps *fakeGoalSnapshotStore, analytics *fakeEvalAnalytics) *goalEvaluatorService {
	// Notification deps default to no-ops; the goals in these tests have no
	// ProgressPercent threshold, so the notification path never runs.
	return NewGoalEvaluatorService(users, goals, snaps, analytics, &fakeGoalAccounts{}, &fakeNotificationStore{}, &fakeTasksClient{})
}

// Targets and spend totals are in integer minor units (e.g. 30000 = $300.00).
func monthlyGoal(id string, targetMinor int64) *models.Goal {
	return &models.Goal{
		GoalID:           id,
		Type:             models.GoalTypeSpendingLimit,
		TargetValueMinor: targetMinor,
		Currency:         helpers.CurrencyUSD,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Status:           models.GoalStatusActive,
	}
}

func TestGoalEvaluator_SnapshotPerActiveGoal(t *testing.T) {
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{
		"g1": monthlyGoal("g1", 30000),
		"g2": monthlyGoal("g2", 10000),
	}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 12000, Currency: "USD"}}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
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
	if s := byGoal["g1"]; s == nil || s.CurrentValueMinor != 12000 || s.TargetValueMinor != 30000 || s.PercentComplete != 40 {
		t.Fatalf("g1 snapshot wrong: %+v", s)
	}
	if s := byGoal["g1"]; s.Currency != helpers.CurrencyUSD {
		t.Fatalf("snapshot currency should be copied from the goal, got %q", s.Currency)
	}
	if s := byGoal["g2"]; s == nil || s.PercentComplete != 120 {
		t.Fatalf("g2 snapshot wrong: %+v", s)
	}
	if byGoal["g1"].CreatedAt != evalClock {
		t.Fatalf("snapshot createdAt should be the run clock, got %s", byGoal["g1"].CreatedAt)
	}
}

func TestGoalEvaluator_MapsFiltersAndWindow(t *testing.T) {
	g := monthlyGoal("g1", 30000)
	g.Filters = models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK", Merchant: "cafe", AccountID: "acc1"}

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 1000}}

	if err := newEvaluator(users, goals, &fakeGoalSnapshotStore{}, analytics).Run(evalContext()); err != nil {
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
	// On 2026-08-15, 15 of 31 days elapsed ≈ 48.4%; target 30000 → pace ≈ 14500.
	cases := []struct {
		name       string
		spentMinor int64
		onTrack    bool
	}{
		{"under pace", 10000, true},
		{"over pace", 20000, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
			goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": monthlyGoal("g1", 30000)}}
			snaps := &fakeGoalSnapshotStore{}
			analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: tc.spentMinor}}

			if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
				t.Fatalf("Run error: %v", err)
			}
			if len(snaps.created) != 1 {
				t.Fatalf("expected 1 snapshot, got %d", len(snaps.created))
			}
			if snaps.created[0].IsOnTrack != tc.onTrack {
				t.Fatalf("spent %v: isOnTrack=%v, want %v", tc.spentMinor, snaps.created[0].IsOnTrack, tc.onTrack)
			}
		})
	}
}

func TestGoalEvaluator_PerGoalErrorIsolation(t *testing.T) {
	// g1 has a fixed window with an unparseable EndDate, so ResolveWindow errors;
	// g2 is a normal monthly goal. The bad goal must not stop the good one.
	bad := &models.Goal{
		GoalID:           "g1",
		Type:             models.GoalTypeSpendingLimit,
		TargetValueMinor: 10000,
		TimeWindow:       models.GoalWindowFixed,
		Recurrence:       models.GoalRecurrenceOneOff,
		EndDate:          "not-a-date",
		Status:           models.GoalStatusActive,
	}
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": bad, "g2": monthlyGoal("g2", 10000)}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 5000}}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run should not fail on a single bad goal: %v", err)
	}
	if len(snaps.created) != 1 || snaps.created[0].GoalID != "g2" {
		t.Fatalf("expected only g2 snapshotted, got %+v", snaps.created)
	}
}

func TestGoalEvaluator_UserListErrorPropagates(t *testing.T) {
	users := &fakeEvalUserStore{err: errors.New("firestore down")}
	svc := newEvaluator(users, &fakeGoalStore{goals: map[string]*models.Goal{}}, &fakeGoalSnapshotStore{}, &fakeEvalAnalytics{})

	if err := svc.Run(evalContext()); err == nil {
		t.Fatal("expected Run to fail when listing users fails")
	}
}

// goalWithThreshold builds a recurring monthly spending-limit goal that fires a
// notification at the given progress percent.
func goalWithThreshold(id string, targetMinor int64, thresholdPct float64) *models.Goal {
	g := monthlyGoal(id, targetMinor)
	g.Name = "Dining"
	g.AlertThresholds = models.GoalAlertThresholds{ProgressPercent: helpers.Ptr(thresholdPct)}
	return g
}

func newNotifyEvaluator(goals *fakeGoalStore, snaps *fakeGoalSnapshotStore, analytics *fakeEvalAnalytics, notifs *fakeNotificationStore, tasks *fakeTasksClient) *goalEvaluatorService {
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	return NewGoalEvaluatorService(users, goals, snaps, analytics, &fakeGoalAccounts{}, notifs, tasks)
}

func TestGoalEvaluator_FiresThresholdNotification(t *testing.T) {
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 30000, 80)}}
	snaps := &fakeGoalSnapshotStore{}                                                                          // no prior snapshots this period
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 25000, Currency: "USD"}} // 83.3% >= 80
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(notifs.created) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs.created))
	}
	n := notifs.created[0]
	if n.Source != models.NotificationSourceGoal || n.SourceID != "g1" {
		t.Fatalf("wrong notification source/id: %+v", n)
	}
	if n.Title != "Dining" || n.Body != "You've used 83.3% of your Dining budget — USD 250.00 of USD 300.00, USD 50.00 remaining." {
		t.Fatalf("wrong title/body: %+v", n)
	}
	if n.Delivery != models.DeliveryPush || n.Data["sourceId"] != "g1" {
		t.Fatalf("wrong delivery/data: %+v", n)
	}
	if len(tasks.enqueued) != 1 || tasks.enqueued[0].NotificationID != n.NotificationID || tasks.enqueued[0].UserID != "u1" {
		t.Fatalf("delivery not enqueued for the notification: %+v", tasks.enqueued)
	}
	if len(snaps.created) != 1 || !snaps.created[0].NotificationSent || snaps.created[0].AIInsight == "" {
		t.Fatalf("snapshot should record the notification + insight: %+v", snaps.created)
	}
}

func TestGoalEvaluator_NoNotificationWhileStayingOver(t *testing.T) {
	// Previous snapshot this period was already over the threshold and we're still
	// over: no state change, so no repeat notification.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 30000, 80)}}
	snaps := &fakeGoalSnapshotStore{sinceSnapshots: []*models.GoalSnapshot{
		{GoalID: "g1", PercentComplete: 82},
	}}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 24600, Currency: "USD"}} // 82%, still over
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 0 || len(tasks.enqueued) != 0 {
		t.Fatalf("expected no repeat notification while staying over, got %d notif / %d enqueued", len(notifs.created), len(tasks.enqueued))
	}
	if len(snaps.created) != 1 || snaps.created[0].NotificationSent {
		t.Fatalf("expected a snapshot with NotificationSent=false, got %+v", snaps.created)
	}
}

func TestGoalEvaluator_NotifiesOnDroppingBackUnder(t *testing.T) {
	// Previous snapshot this period was over the threshold; a refund drops us back
	// under. That downward transition notifies (the "you recovered" signal).
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 30000, 80)}}
	snaps := &fakeGoalSnapshotStore{sinceSnapshots: []*models.GoalSnapshot{
		{GoalID: "g1", PercentComplete: 83},
	}}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 21000, Currency: "USD"}} // 70%, back under
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 1 || len(tasks.enqueued) != 1 {
		t.Fatalf("expected a recovery notification, got %d notif / %d enqueued", len(notifs.created), len(tasks.enqueued))
	}
	if notifs.created[0].Body != "Your Dining spending is back down to 70.0% — USD 210.00 of USD 300.00, USD 90.00 remaining." {
		t.Fatalf("unexpected recovery body: %q", notifs.created[0].Body)
	}
	if len(snaps.created) != 1 || !snaps.created[0].NotificationSent {
		t.Fatalf("expected a snapshot with NotificationSent=true, got %+v", snaps.created)
	}
}

func TestGoalEvaluator_NoNotificationBelowThreshold(t *testing.T) {
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 30000, 80)}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 12000}} // 40% < 80
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 0 {
		t.Fatalf("expected no notification below threshold, got %d", len(notifs.created))
	}
}

// oneOffFixedGoal builds a one-off fixed-window spending-limit goal.
func oneOffFixedGoal(id string, targetMinor int64, created, endDate string) *models.Goal {
	return &models.Goal{
		GoalID:           id,
		Type:             models.GoalTypeSpendingLimit,
		Name:             "Reno",
		TargetValueMinor: targetMinor,
		Currency:         helpers.CurrencyUSD,
		TimeWindow:       models.GoalWindowFixed,
		Recurrence:       models.GoalRecurrenceOneOff,
		Status:           models.GoalStatusActive,
		CreatedAt:        time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		EndDate:          endDate,
	}
}

func TestGoalEvaluator_OneOffUnderBudgetCompletes(t *testing.T) {
	// EndDate is before the run clock (2026-08-15), so the window has closed.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 30000, "2026-07-01", "2026-07-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 25000, Currency: "USD"}} // under 300
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if goals.goals["g1"].Status != models.GoalStatusCompleted {
		t.Fatalf("expected status completed, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "You completed your Reno goal — spent USD 250.00, within your USD 300.00 limit." {
		t.Fatalf("wrong terminal notification: %+v", notifs.created)
	}
	if len(tasks.enqueued) != 1 {
		t.Fatalf("expected delivery enqueued, got %d", len(tasks.enqueued))
	}
	// The final spend query is capped at the window end, not today.
	if helpers.Value(analytics.calls[0].DateTo) != "2026-07-31" {
		t.Fatalf("expected final query capped at endDate, got %q", helpers.Value(analytics.calls[0].DateTo))
	}
}

func TestGoalEvaluator_OneOffOverBudgetFails(t *testing.T) {
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 30000, "2026-07-01", "2026-07-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 35000, Currency: "USD"}} // over 300
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if goals.goals["g1"].Status != models.GoalStatusFailed {
		t.Fatalf("expected status failed, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "Your Reno goal ended over budget — spent USD 350.00 of your USD 300.00 limit." {
		t.Fatalf("wrong terminal notification: %+v", notifs.created)
	}
}

func TestGoalEvaluator_ReductionEvaluatesAgainstFrozenTarget(t *testing.T) {
	// A reduction goal behaves like a spending limit at eval time: its frozen
	// target (baseline 20000, 10% less → 18000) drives percent and pace, and the
	// evaluator never re-measures the baseline.
	baseline := int64(20000)
	g := monthlyGoal("g1", 18000)
	g.Type = models.GoalTypeReduction
	g.ReductionPercent = helpers.Ptr(10.0)
	g.BaselineValueMinor = &baseline

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 9000, Currency: "USD"}} // 50% of 18000

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(snaps.created) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps.created))
	}
	if s := snaps.created[0]; s.TargetValueMinor != 18000 || s.PercentComplete != 50 {
		t.Fatalf("expected snapshot against frozen target 18000 at 50%%, got target=%d pct=%v", s.TargetValueMinor, s.PercentComplete)
	}
	// One spend query for the current window — no second call to re-measure a baseline.
	if len(analytics.calls) != 1 {
		t.Fatalf("expected exactly 1 spend query at eval, got %d", len(analytics.calls))
	}
}

func TestGoalEvaluator_NetSavingsMetScoresAtLeast(t *testing.T) {
	// Net = income 200000 − spend 130000 = 70000 against a 50000 target → 140%,
	// and already met, so on track.
	g := monthlyGoal("g1", 50000)
	g.Type = models.GoalTypeNetSavings

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 200000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 130000, Currency: "USD"},
	}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(snaps.created) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps.created))
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 70000 || s.TargetValueMinor != 50000 || s.PercentComplete != 140 {
		t.Fatalf("expected net 70000 at 140%% of 50000, got current=%d target=%d pct=%v", s.CurrentValueMinor, s.TargetValueMinor, s.PercentComplete)
	}
	if !s.IsOnTrack {
		t.Fatal("expected on track: net already exceeds the target")
	}
	if len(analytics.incomeCalls) != 1 || len(analytics.calls) != 1 {
		t.Fatalf("expected one income and one spend query, got income=%d spend=%d", len(analytics.incomeCalls), len(analytics.calls))
	}
}

func TestGoalEvaluator_NetSavingsBehindPaceNotOnTrack(t *testing.T) {
	// Net = 60000 − 58000 = 2000. On 2026-08-15 the pace line is ~24193
	// (50000 × 15/31), so a thin surplus is behind pace — the at-least direction.
	g := monthlyGoal("g1", 50000)
	g.Type = models.GoalTypeNetSavings

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 60000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 58000, Currency: "USD"},
	}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 2000 {
		t.Fatalf("expected net 2000, got %d", s.CurrentValueMinor)
	}
	if s.IsOnTrack {
		t.Fatal("expected not on track: the surplus is well below the pace line")
	}
}

// netSavingsGoal builds a net savings goal off the monthly template.
func netSavingsGoal(id string, targetMinor int64) *models.Goal {
	g := monthlyGoal(id, targetMinor)
	g.Type = models.GoalTypeNetSavings
	g.Name = "Savings"
	return g
}

func TestGoalEvaluator_NetSavingsTerminalCompletedWording(t *testing.T) {
	// One-off net savings whose window has closed, target reached → completed,
	// phrased for saving (not spending).
	g := oneOffFixedGoal("g1", 50000, "2026-07-01", "2026-07-31")
	g.Type = models.GoalTypeNetSavings
	g.Name = "Summer savings"
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	// net = 300000 − 240000 = 60000 ≥ 50000.
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 300000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 240000, Currency: "USD"},
	}
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if goals.goals["g1"].Status != models.GoalStatusCompleted {
		t.Fatalf("expected completed, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "You reached your Summer savings goal — USD 600.00 of your USD 500.00 target." {
		t.Fatalf("wrong terminal wording: %+v", notifs.created)
	}
}

func TestGoalEvaluator_NetSavingsTerminalFailedWording(t *testing.T) {
	g := oneOffFixedGoal("g1", 50000, "2026-07-01", "2026-07-31")
	g.Type = models.GoalTypeNetSavings
	g.Name = "Summer savings"
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	// net = 250000 − 240000 = 10000 < 50000.
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 250000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 240000, Currency: "USD"},
	}
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if goals.goals["g1"].Status != models.GoalStatusFailed {
		t.Fatalf("expected failed, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "Your Summer savings goal ended short — USD 100.00 of your USD 500.00 target." {
		t.Fatalf("wrong terminal wording: %+v", notifs.created)
	}
}

func TestGoalEvaluator_NetSavingsProgressCrossedOverWording(t *testing.T) {
	g := netSavingsGoal("g1", 50000)
	g.AlertThresholds = models.GoalAlertThresholds{ProgressPercent: helpers.Ptr(80.0)}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{} // no prior snapshot this period
	// net = 250000 − 205000 = 45000 → 90% of 50000, crosses 80%.
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 250000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 205000, Currency: "USD"},
	}
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "You're 90.0% of the way to your Savings goal — USD 450.00 of USD 500.00, USD 50.00 to go." {
		t.Fatalf("wrong progress wording: %+v", notifs.created)
	}
}

func TestGoalEvaluator_NetSavingsProgressSlippedUnderWording(t *testing.T) {
	g := netSavingsGoal("g1", 50000)
	g.AlertThresholds = models.GoalAlertThresholds{ProgressPercent: helpers.Ptr(80.0)}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	// Prior snapshot was above the threshold; a correction drops progress back under.
	snaps := &fakeGoalSnapshotStore{sinceSnapshots: []*models.GoalSnapshot{{GoalID: "g1", PercentComplete: 90}}}
	// net = 235000 − 200000 = 35000 → 70% of 50000, back under 80%.
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 235000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 200000, Currency: "USD"},
	}
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "Your Savings progress slipped to 70.0% — USD 350.00 of USD 500.00, USD 150.00 to go." {
		t.Fatalf("wrong progress wording: %+v", notifs.created)
	}
}

func TestGoalEvaluator_IncomeTargetMeasuresIncomeOnly(t *testing.T) {
	// Income target scores income alone against the floor — it must not subtract
	// spend (that's net savings), so it never queries the spend total.
	g := monthlyGoal("g1", 300000)
	g.Type = models.GoalTypeIncomeTarget
	g.Name = "Freelance income"

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{
		incomeResult: dto.AnalyticsIncomeTotalResult{TotalMinor: 360000, Currency: "USD"},
		result:       dto.AnalyticsSpendTotalResult{TotalMinor: 200000, Currency: "USD"}, // must be ignored
	}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 360000 || s.PercentComplete != 120 {
		t.Fatalf("expected income 360000 at 120%% of 300000, got current=%d pct=%v", s.CurrentValueMinor, s.PercentComplete)
	}
	if !s.IsOnTrack {
		t.Fatal("expected on track: income already exceeds the target")
	}
	if len(analytics.incomeCalls) != 1 || len(analytics.calls) != 0 {
		t.Fatalf("income target must query income only, got income=%d spend=%d", len(analytics.incomeCalls), len(analytics.calls))
	}
}

func TestGoalEvaluator_SavingsTargetMeasuresBalanceDelta(t *testing.T) {
	// Balance grew from an 800000 baseline to 900000 → 100000 saved against a
	// 200000 target = 50%. Measured from the balance, not spend/income, so the
	// analytics client is never touched. On 2026-08-15 (~48% of the window) a
	// 50% delta is ahead of pace.
	baseline := int64(800000)
	g := &models.Goal{
		GoalID:             "g1",
		Type:               models.GoalTypeSavingsTarget,
		Name:               "Save $2k",
		TargetValueMinor:   200000,
		BaselineValueMinor: &baseline,
		Currency:           helpers.CurrencyUSD,
		TimeWindow:         models.GoalWindowFixed,
		Recurrence:         models.GoalRecurrenceOneOff,
		EndDate:            "2026-08-31",
		CreatedAt:          time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Status:             models.GoalStatusActive,
	}

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{}
	accounts := &fakeGoalAccounts{balance: 900000}
	svc := NewGoalEvaluatorService(users, goals, snaps, analytics, accounts, &fakeNotificationStore{}, &fakeTasksClient{})

	if err := svc.Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 100000 || s.TargetValueMinor != 200000 || s.PercentComplete != 50 {
		t.Fatalf("expected 100000 saved at 50%% of 200000, got current=%d target=%d pct=%v", s.CurrentValueMinor, s.TargetValueMinor, s.PercentComplete)
	}
	if !s.IsOnTrack {
		t.Fatal("expected on track: 50%% saved is ahead of the ~48%% window pace")
	}
	if len(analytics.calls) != 0 || len(analytics.incomeCalls) != 0 {
		t.Fatalf("a balance goal must not query spend/income, got spend=%d income=%d", len(analytics.calls), len(analytics.incomeCalls))
	}
	if len(accounts.calls) != 1 || accounts.calls[0] != nil {
		t.Fatalf("expected one unscoped balance read (nil accountId), got %v", accounts.calls)
	}
}

func TestGoalEvaluator_SavingsContributionsMeasuresTransfersInScoped(t *testing.T) {
	// A recurring monthly contributions goal: $600 paid in against a $500 target =
	// 120%, scoped to the destination account. It's a flow, so it reads the
	// contributions total (transfers in), never spend/income or the balance.
	g := monthlyGoal("g1", 50000)
	g.Type = models.GoalTypeSavingsContributions
	g.Name = "Pay into savings"
	g.Filters = models.GoalFilters{AccountID: "acc-savings"}

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{contribResult: dto.AnalyticsContributionsTotalResult{TotalMinor: 60000, Currency: "USD"}}

	if err := newEvaluator(users, goals, snaps, analytics).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 60000 || s.TargetValueMinor != 50000 || s.PercentComplete != 120 {
		t.Fatalf("expected 60000 contributed at 120%% of 50000, got current=%d target=%d pct=%v", s.CurrentValueMinor, s.TargetValueMinor, s.PercentComplete)
	}
	if !s.IsOnTrack {
		t.Fatal("expected on track: contributions already exceed the target")
	}
	if len(analytics.contribCalls) != 1 {
		t.Fatalf("expected one contributions query, got %d", len(analytics.contribCalls))
	}
	if helpers.Value(analytics.contribCalls[0].AccountID) != "acc-savings" {
		t.Fatalf("expected the contributions query scoped to acc-savings, got %v", analytics.contribCalls[0].AccountID)
	}
	if len(analytics.calls) != 0 || len(analytics.incomeCalls) != 0 {
		t.Fatalf("a contributions goal must not query spend/income, got spend=%d income=%d", len(analytics.calls), len(analytics.incomeCalls))
	}
}

func TestGoalEvaluator_PayDownMeasuresDebtReducedScoped(t *testing.T) {
	// A one-off pay down: clear $2000 of a card that owed $5000 at creation. The
	// balance now reads $3500 owed, so $1500 has been paid off = 75%. It's a
	// balance goal scoped to the debt account, so it never queries spend/income.
	baseline := int64(500000)
	g := &models.Goal{
		GoalID:             "g1",
		Type:               models.GoalTypePayDown,
		Name:               "Clear the card",
		TargetValueMinor:   200000,
		BaselineValueMinor: &baseline,
		Currency:           helpers.CurrencyUSD,
		TimeWindow:         models.GoalWindowFixed,
		Recurrence:         models.GoalRecurrenceOneOff,
		EndDate:            "2026-08-31",
		CreatedAt:          time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Status:             models.GoalStatusActive,
		Filters:            models.GoalFilters{AccountID: "acc-card"},
	}

	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{}
	accounts := &fakeGoalAccounts{balance: 350000} // $3,500 still owed
	svc := NewGoalEvaluatorService(users, goals, snaps, analytics, accounts, &fakeNotificationStore{}, &fakeTasksClient{})

	if err := svc.Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	s := snaps.created[0]
	if s.CurrentValueMinor != 150000 || s.TargetValueMinor != 200000 || s.PercentComplete != 75 {
		t.Fatalf("expected 150000 paid off at 75%% of 200000, got current=%d target=%d pct=%v", s.CurrentValueMinor, s.TargetValueMinor, s.PercentComplete)
	}
	if len(analytics.calls) != 0 || len(analytics.incomeCalls) != 0 {
		t.Fatalf("a pay down goal must not query spend/income, got spend=%d income=%d", len(analytics.calls), len(analytics.incomeCalls))
	}
	if len(accounts.calls) != 1 || accounts.calls[0] == nil || *accounts.calls[0] != "acc-card" {
		t.Fatalf("expected the balance read scoped to acc-card, got %v", accounts.calls)
	}
}

func TestGoalEvaluator_EmergencyFundUntilReached(t *testing.T) {
	// Target is pre-frozen ($6,000); the evaluator only reads the balance. It's an
	// until_reached goal, so it completes when the balance reaches the target and
	// stays active (never fails) below it. Measures the absolute balance, scoped to
	// the fund account, and never queries spend/income.
	mkGoal := func() *models.Goal {
		return &models.Goal{
			GoalID:           "g1",
			Type:             models.GoalTypeEmergencyFund,
			Name:             "Emergency fund",
			TargetValueMinor: 600000,
			Currency:         helpers.CurrencyUSD,
			TimeWindow:       models.GoalWindowUntilReached,
			Recurrence:       models.GoalRecurrenceOneOff,
			CreatedAt:        time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			Status:           models.GoalStatusActive,
			Filters:          models.GoalFilters{AccountID: "acc-savings"},
		}
	}

	t.Run("below target stays active", func(t *testing.T) {
		goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": mkGoal()}}
		snaps := &fakeGoalSnapshotStore{}
		analytics := &fakeEvalAnalytics{}
		accounts := &fakeGoalAccounts{balance: 300000} // half way
		svc := NewGoalEvaluatorService(&fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}, goals, snaps, analytics, accounts, &fakeNotificationStore{}, &fakeTasksClient{})

		if err := svc.Run(evalContext()); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		s := snaps.created[0]
		if s.CurrentValueMinor != 300000 || s.PercentComplete != 50 {
			t.Fatalf("expected 300000 at 50%%, got current=%d pct=%v", s.CurrentValueMinor, s.PercentComplete)
		}
		if !s.IsOnTrack {
			t.Fatal("expected on track: no deadline means never behind")
		}
		if goals.goals["g1"].Status != models.GoalStatusActive {
			t.Fatalf("expected still active below target, got %s", goals.goals["g1"].Status)
		}
		if len(analytics.calls) != 0 || len(analytics.incomeCalls) != 0 {
			t.Fatalf("a balance goal must not query spend/income, got spend=%d income=%d", len(analytics.calls), len(analytics.incomeCalls))
		}
		if len(accounts.calls) != 1 || accounts.calls[0] == nil || *accounts.calls[0] != "acc-savings" {
			t.Fatalf("expected balance read scoped to acc-savings, got %v", accounts.calls)
		}
	})

	t.Run("reaching target completes and notifies", func(t *testing.T) {
		goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": mkGoal()}}
		snaps := &fakeGoalSnapshotStore{}
		notifs := &fakeNotificationStore{}
		tasks := &fakeTasksClient{}
		accounts := &fakeGoalAccounts{balance: 600000} // reached
		svc := NewGoalEvaluatorService(&fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}, goals, snaps, &fakeEvalAnalytics{}, accounts, notifs, tasks)

		if err := svc.Run(evalContext()); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if goals.goals["g1"].Status != models.GoalStatusCompleted {
			t.Fatalf("expected completed on reaching target, got %s", goals.goals["g1"].Status)
		}
		if len(notifs.created) != 1 {
			t.Fatalf("expected one terminal notification, got %d", len(notifs.created))
		}
		if !snaps.created[0].NotificationSent {
			t.Fatal("expected the snapshot to record the terminal notification")
		}
	})
}

func TestGoalEvaluator_OneOffNotYetEndedDoesNotTerminate(t *testing.T) {
	// EndDate is in the future, so the goal stays active — no terminal transition.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 30000, "2026-07-01", "2026-12-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 35000, Currency: "USD"}} // over, but window still open
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(evalContext()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if goals.goals["g1"].Status != models.GoalStatusActive {
		t.Fatalf("expected status to remain active, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 0 {
		t.Fatalf("expected no terminal notification while the window is open, got %d", len(notifs.created))
	}
	if len(snaps.created) != 1 {
		t.Fatalf("expected a snapshot to still be written, got %d", len(snaps.created))
	}
}
