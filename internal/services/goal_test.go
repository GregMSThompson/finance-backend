package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

func goalContextAt(now time.Time) context.Context {
	return clock.WithClock(context.Background(), func() time.Time { return now })
}

// --- fakes ---

type fakeGoalStore struct {
	goals            map[string]*models.Goal
	getErr           error
	updateErr        error
	deleteErr        error
	deleted          []string
	lastListStatuses []models.GoalStatus
}

func newFakeGoalStore() *fakeGoalStore {
	return &fakeGoalStore{goals: map[string]*models.Goal{}}
}

func (f *fakeGoalStore) Create(_ context.Context, _ string, g *models.Goal) error {
	f.goals[g.GoalID] = g
	return nil
}

func (f *fakeGoalStore) Get(_ context.Context, _, goalID string) (*models.Goal, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	g, ok := f.goals[goalID]
	if !ok {
		return nil, errs.NewNotFoundError("goal not found")
	}
	return g, nil
}

func (f *fakeGoalStore) List(_ context.Context, _ string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	f.lastListStatuses = statuses
	out := make([]*models.Goal, 0, len(f.goals))
	for _, g := range f.goals {
		out = append(out, g)
	}
	return out, nil
}

func (f *fakeGoalStore) Update(_ context.Context, _ string, g *models.Goal) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.goals[g.GoalID] = g
	return nil
}

func (f *fakeGoalStore) Delete(_ context.Context, _, goalID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, goalID)
	delete(f.goals, goalID)
	return nil
}

type fakeGoalSnapshotStore struct {
	latest         *models.GoalSnapshot
	latestErr      error
	forGoal        []*models.GoalSnapshot
	lastListLimit  int
	deletedForGoal []string
	created        []*models.GoalSnapshot
	createErr      error
	sinceSnapshots []*models.GoalSnapshot
	sinceErr       error
}

func (f *fakeGoalSnapshotStore) Create(_ context.Context, _ string, snap *models.GoalSnapshot) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, snap)
	return nil
}

func (f *fakeGoalSnapshotStore) ListForGoalSince(_ context.Context, _, _ string, _ time.Time) ([]*models.GoalSnapshot, error) {
	return f.sinceSnapshots, f.sinceErr
}

func (f *fakeGoalSnapshotStore) Latest(_ context.Context, _, _ string) (*models.GoalSnapshot, error) {
	return f.latest, f.latestErr
}

func (f *fakeGoalSnapshotStore) ListForGoal(_ context.Context, _, _ string, limit int) ([]*models.GoalSnapshot, error) {
	f.lastListLimit = limit
	return f.forGoal, nil
}

func (f *fakeGoalSnapshotStore) DeleteForGoal(_ context.Context, _, goalID string) error {
	f.deletedForGoal = append(f.deletedForGoal, goalID)
	return nil
}

// --- helpers ---

func validGoalDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeSpendingLimit,
		Name:             "Dining — Monthly",
		TargetValueMinor: 22000,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Filters:          models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK"},
	}
}

func validReductionDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeReduction,
		Name:             "Dining — 10% less",
		ReductionPercent: helpers.Ptr(10.0),
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Filters:          models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK"},
	}
}

func seedGoal(store *fakeGoalStore) *models.Goal {
	g := &models.Goal{
		GoalID:           "g1",
		Type:             models.GoalTypeSpendingLimit,
		Name:             "Dining — Monthly",
		TargetValueMinor: 22000,
		Currency:         helpers.CurrencyUSD,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Status:           models.GoalStatusActive,
	}
	store.goals[g.GoalID] = g
	return g
}

// --- Create ---

func TestGoalCreate_Valid(t *testing.T) {
	goals := newFakeGoalStore()
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	g, err := svc.Create(context.Background(), "uid1", "session-1", validGoalDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.GoalID == "" {
		t.Fatal("expected GoalID to be set")
	}
	if g.Status != models.GoalStatusActive {
		t.Fatalf("expected active status, got %s", g.Status)
	}
	if g.ConversationID != "session-1" {
		t.Fatalf("expected conversationId=session-1, got %q", g.ConversationID)
	}
	if g.Currency != helpers.CurrencyUSD {
		t.Fatalf("expected currency pinned to USD, got %q", g.Currency)
	}
}

func TestGoalCreate_ZeroTarget(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})
	def := validGoalDef()
	def.TargetValueMinor = 0
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestGoalCreate_RecurringFixedRejected(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})
	def := validGoalDef()
	def.TimeWindow = models.GoalWindowFixed
	def.EndDate = "2026-12-31"
	// recurrence stays recurring — invalid combo
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for recurring+fixed, got %v", err)
	}
}

func TestGoalCreate_FixedRequiresEndDate(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})
	def := validGoalDef()
	def.TimeWindow = models.GoalWindowFixed
	def.Recurrence = models.GoalRecurrenceOneOff
	def.EndDate = "" // missing
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for fixed without endDate, got %v", err)
	}
}

func TestGoalCreate_InvalidCategory(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})
	def := validGoalDef()
	def.Filters.PFCPrimary = "NOT_A_CATEGORY"
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for bad category, got %v", err)
	}
}

func TestGoalCreate_ReductionFreezesTargetFromBaseline(t *testing.T) {
	goals := newFakeGoalStore()
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 20000, Currency: "USD"}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics)

	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	g, err := svc.Create(goalContextAt(now), "uid1", "session-1", validReductionDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.BaselineValueMinor == nil || *g.BaselineValueMinor != 20000 {
		t.Fatalf("expected baseline 20000 captured, got %v", g.BaselineValueMinor)
	}
	if g.TargetValueMinor != 18000 {
		t.Fatalf("expected target frozen to baseline×(1−10%%)=18000, got %d", g.TargetValueMinor)
	}
	if len(analytics.calls) != 1 {
		t.Fatalf("expected 1 baseline spend query, got %d", len(analytics.calls))
	}
	args := analytics.calls[0]
	// Created mid-August, so the comparable prior period is all of July.
	if helpers.Value(args.DateFrom) != "2026-07-01" || helpers.Value(args.DateTo) != "2026-07-31" {
		t.Fatalf("baseline should query the previous calendar month, got from=%q to=%q", helpers.Value(args.DateFrom), helpers.Value(args.DateTo))
	}
	if helpers.Value(args.PFCPrimary) != "FOOD_AND_DRINK" {
		t.Fatalf("baseline query should carry the goal's filters, got %+v", args)
	}
}

func TestGoalCreate_ReductionRequiresPercent(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})
	def := validReductionDef()
	def.ReductionPercent = nil
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError when reductionPercent missing, got %v", err)
	}
}

// --- Update ---

func TestGoalUpdate_PartialMerge(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	updated, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		TargetValueMinor: helpers.Ptr(int64(30000)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.TargetValueMinor != 30000 {
		t.Fatalf("expected target 30000, got %v", updated.TargetValueMinor)
	}
	if updated.Name != "Dining — Monthly" {
		t.Fatalf("name should be unchanged, got %q", updated.Name)
	}
}

func TestGoalUpdate_RevalidatesMergedGoal(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	_, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		TargetValueMinor: helpers.Ptr(int64(-500)),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for negative target, got %v", err)
	}
}

func TestGoalUpdate_PauseOK(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	updated, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		Status: helpers.Ptr(models.GoalStatusPaused),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != models.GoalStatusPaused {
		t.Fatalf("expected paused, got %s", updated.Status)
	}
}

func TestGoalUpdate_CompletedRejected(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	_, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		Status: helpers.Ptr(models.GoalStatusCompleted),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for manual completed, got %v", err)
	}
}

// --- Delete ---

func TestGoalDelete_SubmitsJob(t *testing.T) {
	jobs := &fakeJobs{jobID: "job-xyz"}
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, jobs, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	got, err := svc.Delete(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "job-xyz" {
		t.Fatalf("expected jobID job-xyz, got %q", got)
	}
	if jobs.gotTyp != models.JobTypeGoalDelete {
		t.Fatalf("expected JobTypeGoalDelete, got %s", jobs.gotTyp)
	}
	var params dto.GoalDeleteParams
	if err := json.Unmarshal(jobs.gotRaw, &params); err != nil {
		t.Fatalf("params not valid json: %v", err)
	}
	if params.GoalID != "g1" {
		t.Fatalf("expected params.GoalID=g1, got %q", params.GoalID)
	}
}

func TestGoalRunDelete_Cascades(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	snaps := &fakeGoalSnapshotStore{}
	svc := NewGoalService(goals, snaps, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	_, err := svc.RunDelete(context.Background(), "uid1", dto.GoalDeleteParams{GoalID: "g1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps.deletedForGoal) != 1 || snaps.deletedForGoal[0] != "g1" {
		t.Fatalf("expected snapshots deleted for g1, got %v", snaps.deletedForGoal)
	}
	if len(goals.deleted) != 1 || goals.deleted[0] != "g1" {
		t.Fatalf("expected goal g1 deleted, got %v", goals.deleted)
	}
}

// --- GetProgress ---

func TestGoalGetProgress_NoSnapshot(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{latest: nil}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	prog, err := svc.GetProgress(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog.CurrentValueMinor != 0 || prog.AmountRemainingMinor != 22000 {
		t.Fatalf("expected zero progress, remaining=22000, got current=%v remaining=%v", prog.CurrentValueMinor, prog.AmountRemainingMinor)
	}
	if !prog.IsOnTrack {
		t.Fatal("expected isOnTrack=true for un-evaluated goal")
	}
	if !prog.AsOf.IsZero() {
		t.Fatalf("expected zero AsOf for un-evaluated goal, got %v", prog.AsOf)
	}
}

func TestGoalGetProgress_WithSnapshot(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	at := time.Date(2026, time.July, 21, 2, 0, 0, 0, time.UTC)
	snaps := &fakeGoalSnapshotStore{latest: &models.GoalSnapshot{
		CurrentValueMinor: 15000,
		PercentComplete:   68.2,
		IsOnTrack:         true,
		AIInsight:         "You're on track.",
		CreatedAt:         at,
	}}
	svc := NewGoalService(goals, snaps, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{})

	prog, err := svc.GetProgress(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog.CurrentValueMinor != 15000 || prog.AmountRemainingMinor != 7000 {
		t.Fatalf("expected current=15000 remaining=7000, got current=%v remaining=%v", prog.CurrentValueMinor, prog.AmountRemainingMinor)
	}
	if prog.PercentComplete != 68.2 || prog.AIInsight != "You're on track." {
		t.Fatalf("unexpected progress fields: %+v", prog)
	}
	if prog.Currency != helpers.CurrencyUSD {
		t.Fatalf("expected progress currency copied from the goal, got %q", prog.Currency)
	}
	if !prog.AsOf.Equal(at) {
		t.Fatalf("expected AsOf=%v, got %v", at, prog.AsOf)
	}
}

func TestListGoalTransactions_ScopesToWindowAndFilters(t *testing.T) {
	g := &models.Goal{
		GoalID:           "g1",
		Type:             models.GoalTypeSpendingLimit,
		TargetValueMinor: 30000,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Status:           models.GoalStatusActive,
		Filters:          models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK", Merchant: "cafe", AccountID: "acc1"},
	}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	tx := &fakeTransactionsLister{resp: dto.TransactionListResult{
		Transactions: []models.Transaction{{TransactionID: "t1"}},
	}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, tx, &fakeEvalAnalytics{})

	cursor := "cur"
	res, err := svc.ListGoalTransactions(goalContextAt(time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)), "uid1", "g1", &cursor, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Transactions) != 1 || res.Transactions[0].TransactionID != "t1" {
		t.Fatalf("expected the lister's result to pass through, got %+v", res)
	}

	args := tx.args
	if helpers.Value(args.DateFrom) != "2026-08-01" || helpers.Value(args.DateTo) != "2026-08-15" {
		t.Fatalf("window mismatch: from=%q to=%q", helpers.Value(args.DateFrom), helpers.Value(args.DateTo))
	}
	if len(args.PFCPrimaries) != 1 || args.PFCPrimaries[0] != "FOOD_AND_DRINK" {
		t.Fatalf("category filter not mapped: %v", args.PFCPrimaries)
	}
	if helpers.Value(args.Merchant) != "cafe" || helpers.Value(args.AccountID) != "acc1" {
		t.Fatalf("merchant/account filters not mapped: %+v", args)
	}
	if args.OrderBy != "date" || !args.Desc {
		t.Fatalf("expected order by date desc, got orderBy=%q desc=%v", args.OrderBy, args.Desc)
	}
	if helpers.Value(args.Pending) != false {
		t.Fatalf("expected pending=false")
	}
	if args.Cursor != &cursor || args.Limit != 25 {
		t.Fatalf("pagination not forwarded: cursor=%v limit=%d", args.Cursor, args.Limit)
	}
}

func TestListGoalTransactions_CapsAtWindowEnd(t *testing.T) {
	// A fixed window that closed in the past: DateTo must be the endDate, not today.
	g := &models.Goal{
		GoalID:           "g1",
		Type:             models.GoalTypeSpendingLimit,
		TargetValueMinor: 200000,
		TimeWindow:       models.GoalWindowFixed,
		Recurrence:       models.GoalRecurrenceOneOff,
		Status:           models.GoalStatusActive,
		CreatedAt:        time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		EndDate:          "2026-06-30",
	}
	goals := &fakeGoalStore{goals: map[string]*models.Goal{"g1": g}}
	tx := &fakeTransactionsLister{}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, tx, &fakeEvalAnalytics{})

	if _, err := svc.ListGoalTransactions(goalContextAt(time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)), "uid1", "g1", nil, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if helpers.Value(tx.args.DateFrom) != "2026-06-01" || helpers.Value(tx.args.DateTo) != "2026-06-30" {
		t.Fatalf("expected window capped at endDate, got from=%q to=%q", helpers.Value(tx.args.DateFrom), helpers.Value(tx.args.DateTo))
	}
}
