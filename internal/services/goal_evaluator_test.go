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
	// Notification deps default to no-ops; the goals in these tests have no
	// ProgressPercent threshold, so the notification path never runs.
	svc := NewGoalEvaluatorService(users, goals, snaps, analytics, &fakeNotificationStore{}, &fakeTasksClient{})
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

// goalWithThreshold builds a recurring monthly spending-limit goal that fires a
// notification at the given progress percent.
func goalWithThreshold(id string, target, thresholdPct float64) *models.Goal {
	g := monthlyGoal(id, target)
	g.Name = "Dining"
	g.AlertThresholds = models.GoalAlertThresholds{ProgressPercent: helpers.Ptr(thresholdPct)}
	return g
}

func newNotifyEvaluator(goals *fakeGoalStore, snaps *fakeGoalSnapshotStore, analytics *fakeEvalAnalytics, notifs *fakeNotificationStore, tasks *fakeTasksClient) *goalEvaluatorService {
	users := &fakeEvalUserStore{users: []*models.User{{UID: "u1"}}}
	svc := NewGoalEvaluatorService(users, goals, snaps, analytics, notifs, tasks)
	svc.clockNow = func() time.Time { return evalClock }
	return svc
}

func TestGoalEvaluator_FiresThresholdNotification(t *testing.T) {
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 300, 80)}}
	snaps := &fakeGoalSnapshotStore{}                                                                   // no prior snapshots this period
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 250, Currency: "USD"}} // 83.3% >= 80
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
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

func TestGoalEvaluator_SuppressesReCrossWithinPeriod(t *testing.T) {
	// A prior snapshot this period already reached 80%; a refund dip then re-cross
	// must not notify again.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 300, 80)}}
	snaps := &fakeGoalSnapshotStore{sinceSnapshots: []*models.GoalSnapshot{
		{GoalID: "g1", PercentComplete: 82},
	}}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 246, Currency: "USD"}} // 82% again
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 0 || len(tasks.enqueued) != 0 {
		t.Fatalf("expected no re-notification within the period, got %d notif / %d enqueued", len(notifs.created), len(tasks.enqueued))
	}
	// Snapshot still written, just without a notification.
	if len(snaps.created) != 1 || snaps.created[0].NotificationSent {
		t.Fatalf("expected a snapshot with NotificationSent=false, got %+v", snaps.created)
	}
}

func TestGoalEvaluator_NoNotificationBelowThreshold(t *testing.T) {
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": goalWithThreshold("g1", 300, 80)}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 120}} // 40% < 80
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(notifs.created) != 0 {
		t.Fatalf("expected no notification below threshold, got %d", len(notifs.created))
	}
}

// oneOffFixedGoal builds a one-off fixed-window spending-limit goal.
func oneOffFixedGoal(id string, target float64, created, endDate string) *models.Goal {
	return &models.Goal{
		GoalID:      id,
		Type:        models.GoalTypeSpendingLimit,
		Name:        "Reno",
		TargetValue: target,
		TimeWindow:  models.GoalWindowFixed,
		Recurrence:  models.GoalRecurrenceOneOff,
		Status:      models.GoalStatusActive,
		CreatedAt:   time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     endDate,
	}
}

func TestGoalEvaluator_OneOffUnderBudgetCompletes(t *testing.T) {
	// EndDate is before the run clock (2026-08-15), so the window has closed.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 300, "2026-07-01", "2026-07-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 250, Currency: "USD"}} // under 300
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
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
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 300, "2026-07-01", "2026-07-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 350, Currency: "USD"}} // over 300
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if goals.goals["g1"].Status != models.GoalStatusFailed {
		t.Fatalf("expected status failed, got %s", goals.goals["g1"].Status)
	}
	if len(notifs.created) != 1 || notifs.created[0].Body != "Your Reno goal ended over budget — spent USD 350.00 of your USD 300.00 limit." {
		t.Fatalf("wrong terminal notification: %+v", notifs.created)
	}
}

func TestGoalEvaluator_OneOffNotYetEndedDoesNotTerminate(t *testing.T) {
	// EndDate is in the future, so the goal stays active — no terminal transition.
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": oneOffFixedGoal("g1", 300, "2026-07-01", "2026-12-31")}}
	snaps := &fakeGoalSnapshotStore{}
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{Total: 350, Currency: "USD"}} // over, but window still open
	notifs := &fakeNotificationStore{}
	tasks := &fakeTasksClient{}

	if err := newNotifyEvaluator(goals, snaps, analytics, notifs, tasks).Run(helpers.TestCtx()); err != nil {
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
