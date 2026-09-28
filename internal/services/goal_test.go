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

func validNetSavingsDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeNetSavings,
		Name:             "Save $500 a month",
		TargetValueMinor: 50000,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
	}
}

func validIncomeTargetDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeIncomeTarget,
		Name:             "Earn $3k a month",
		TargetValueMinor: 300000,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
	}
}

func validSavingsTargetDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeSavingsTarget,
		Name:             "Save $2k by year end",
		TargetValueMinor: 200000,
		TimeWindow:       models.GoalWindowFixed,
		Recurrence:       models.GoalRecurrenceOneOff,
		EndDate:          "2026-12-31",
	}
}

func validSavingsContributionsDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypeSavingsContributions,
		Name:             "Pay $500 into savings monthly",
		TargetValueMinor: 50000,
		TimeWindow:       models.GoalWindowMonthly,
		Recurrence:       models.GoalRecurrenceRecurring,
		Filters:          models.GoalFilters{AccountID: "acc-savings"},
	}
}

func validPayDownDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:             models.GoalTypePayDown,
		Name:             "Clear the card",
		TargetValueMinor: 200000,
		TimeWindow:       models.GoalWindowFixed,
		Recurrence:       models.GoalRecurrenceOneOff,
		EndDate:          "2026-12-31",
		Filters:          models.GoalFilters{AccountID: "acc-card"},
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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validGoalDef()
	def.TargetValueMinor = 0
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestGoalCreate_RecurringFixedRejected(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
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
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
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
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validGoalDef()
	def.Filters.PFCPrimary = "NOT_A_CATEGORY"
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for bad category, got %v", err)
	}
}

func TestGoalCreate_SpendingLimitRejectsNonSpendCategory(t *testing.T) {
	// A spending goal filtered to a non-spend category (income or a transfer)
	// would measure the wrong thing — GetSpendTotal honors an explicit category
	// filter and would report the raw income/transfer sum. Both spend types reject
	// it, even though the category itself is a valid PFC primary.
	for _, tc := range []struct {
		name string
		def  func() dto.GoalDefinition
	}{
		{"spending_limit", validGoalDef},
		{"reduction", validReductionDef},
	} {
		for _, cat := range []string{"INCOME", "TRANSFER_IN", "TRANSFER_OUT"} {
			t.Run(tc.name+"/"+cat, func(t *testing.T) {
				svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
				def := tc.def()
				def.Filters.PFCPrimary = cat
				_, err := svc.Create(context.Background(), "uid1", "s", def)
				if !isValidationError(err) {
					t.Fatalf("expected ValidationError for non-spend category %s, got %v", cat, err)
				}
			})
		}
	}
}

func TestGoalCreate_ReductionFreezesTargetFromBaseline(t *testing.T) {
	goals := newFakeGoalStore()
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 20000, Currency: "USD"}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics, &fakeGoalAccounts{})

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
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validReductionDef()
	def.ReductionPercent = nil
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError when reductionPercent missing, got %v", err)
	}
}

func seedReductionGoal(store *fakeGoalStore) *models.Goal {
	baseline := int64(20000)
	g := &models.Goal{
		GoalID:             "r1",
		Type:               models.GoalTypeReduction,
		Name:               "Dining — 10% less",
		TargetValueMinor:   18000,
		BaselineValueMinor: &baseline,
		ReductionPercent:   helpers.Ptr(10.0),
		Currency:           helpers.CurrencyUSD,
		TimeWindow:         models.GoalWindowMonthly,
		Recurrence:         models.GoalRecurrenceRecurring,
		Status:             models.GoalStatusActive,
		// Created mid-August, so the comparable prior period is all of July.
		CreatedAt: time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC),
	}
	store.goals[g.GoalID] = g
	return g
}

func TestGoalCreate_NetSavingsValid(t *testing.T) {
	goals := newFakeGoalStore()
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

	g, err := svc.Create(context.Background(), "uid1", "s", validNetSavingsDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Type != models.GoalTypeNetSavings || g.TargetValueMinor != 50000 {
		t.Fatalf("unexpected goal: %+v", g)
	}
}

func TestGoalCreate_NetSavingsRejectsFilters(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validNetSavingsDef()
	def.Filters = models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK"}
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for filters on a net savings goal, got %v", err)
	}
}

func TestGoalCreate_IncomeTargetValid(t *testing.T) {
	goals := newFakeGoalStore()
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

	g, err := svc.Create(context.Background(), "uid1", "s", validIncomeTargetDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Type != models.GoalTypeIncomeTarget || g.TargetValueMinor != 300000 {
		t.Fatalf("unexpected goal: %+v", g)
	}
}

func TestGoalCreate_IncomeTargetRejectsFilters(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validIncomeTargetDef()
	def.Filters = models.GoalFilters{AccountID: "acc1"}
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for filters on an income target goal, got %v", err)
	}
}

func TestGoalCreate_SavingsTargetFreezesBaselineBalance(t *testing.T) {
	goals := newFakeGoalStore()
	accounts := &fakeGoalAccounts{balance: 800000} // $8,000 already saved
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, accounts)

	g, err := svc.Create(context.Background(), "uid1", "s", validSavingsTargetDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.BaselineValueMinor == nil || *g.BaselineValueMinor != 800000 {
		t.Fatalf("expected baseline balance 800000 captured, got %v", g.BaselineValueMinor)
	}
	// The target stays the delta to save — it is not rewritten from the baseline.
	if g.TargetValueMinor != 200000 {
		t.Fatalf("expected target to remain 200000, got %d", g.TargetValueMinor)
	}
}

func TestGoalCreate_SavingsTargetScopesBaselineToAccount(t *testing.T) {
	accounts := &fakeGoalAccounts{balance: 5000}
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, accounts)

	def := validSavingsTargetDef()
	def.Filters = models.GoalFilters{AccountID: "acc-savings"}
	if _, err := svc.Create(context.Background(), "uid1", "s", def); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts.calls) != 1 || accounts.calls[0] == nil || *accounts.calls[0] != "acc-savings" {
		t.Fatalf("expected baseline scoped to acc-savings, got %v", accounts.calls)
	}
}

func TestGoalCreate_SavingsTargetRejectsRecurring(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validSavingsTargetDef()
	def.TimeWindow = models.GoalWindowMonthly
	def.Recurrence = models.GoalRecurrenceRecurring
	def.EndDate = ""
	if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
		t.Fatalf("expected ValidationError for a recurring savings target, got %v", err)
	}
}

func TestGoalCreate_SavingsTargetRejectsCategoryAndMerchantFilters(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filters models.GoalFilters
	}{
		{"category", models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK"}},
		{"merchant", models.GoalFilters{Merchant: "Amazon"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
			def := validSavingsTargetDef()
			def.Filters = tc.filters
			if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
				t.Fatalf("expected ValidationError for %s filter, got %v", tc.name, err)
			}
		})
	}
}

func TestGoalCreate_SavingsTargetAllowsAccountFilter(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validSavingsTargetDef()
	def.Filters = models.GoalFilters{AccountID: "acc1"}
	if _, err := svc.Create(context.Background(), "uid1", "s", def); err != nil {
		t.Fatalf("an accountId scope should be allowed on a savings target, got %v", err)
	}
}

func TestGoalCreate_SavingsContributionsValid(t *testing.T) {
	goals := newFakeGoalStore()
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

	g, err := svc.Create(context.Background(), "uid1", "s", validSavingsContributionsDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Type != models.GoalTypeSavingsContributions || g.TargetValueMinor != 50000 {
		t.Fatalf("unexpected goal: %+v", g)
	}
	// A flow goal captures no baseline.
	if g.BaselineValueMinor != nil {
		t.Fatalf("expected no baseline for a contributions goal, got %v", g.BaselineValueMinor)
	}
}

func TestGoalCreate_SavingsContributionsRequiresAccount(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validSavingsContributionsDef()
	def.Filters = models.GoalFilters{}
	if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
		t.Fatalf("expected ValidationError when accountId missing, got %v", err)
	}
}

func TestGoalCreate_SavingsContributionsRejectsCategoryAndMerchant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filters models.GoalFilters
	}{
		{"category", models.GoalFilters{AccountID: "acc1", PFCPrimary: "FOOD_AND_DRINK"}},
		{"merchant", models.GoalFilters{AccountID: "acc1", Merchant: "Amazon"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
			def := validSavingsContributionsDef()
			def.Filters = tc.filters
			if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
				t.Fatalf("expected ValidationError for %s filter, got %v", tc.name, err)
			}
		})
	}
}

func TestGoalCreate_SavingsContributionsAllowsRecurring(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	// The default def is already recurring/monthly; assert it's accepted (unlike
	// savings_target, which rejects recurring).
	if _, err := svc.Create(context.Background(), "uid1", "s", validSavingsContributionsDef()); err != nil {
		t.Fatalf("a recurring savings contributions goal should be allowed, got %v", err)
	}
}

func TestGoalCreate_PayDownFreezesBaselineOwed(t *testing.T) {
	goals := newFakeGoalStore()
	accounts := &fakeGoalAccounts{balance: 500000} // $5,000 owed on the card
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, accounts)

	g, err := svc.Create(context.Background(), "uid1", "s", validPayDownDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.BaselineValueMinor == nil || *g.BaselineValueMinor != 500000 {
		t.Fatalf("expected baseline owed 500000 captured, got %v", g.BaselineValueMinor)
	}
	// The target stays the amount to pay off — it is not rewritten from the baseline.
	if g.TargetValueMinor != 200000 {
		t.Fatalf("expected target to remain 200000, got %d", g.TargetValueMinor)
	}
	if len(accounts.calls) != 1 || accounts.calls[0] == nil || *accounts.calls[0] != "acc-card" {
		t.Fatalf("expected baseline scoped to acc-card, got %v", accounts.calls)
	}
}

func TestGoalCreate_PayDownRejectsRecurring(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validPayDownDef()
	def.TimeWindow = models.GoalWindowMonthly
	def.Recurrence = models.GoalRecurrenceRecurring
	def.EndDate = ""
	if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
		t.Fatalf("expected ValidationError for a recurring pay down goal, got %v", err)
	}
}

func TestGoalCreate_PayDownRequiresAccount(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
	def := validPayDownDef()
	def.Filters = models.GoalFilters{}
	if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
		t.Fatalf("expected ValidationError when accountId missing, got %v", err)
	}
}

func TestGoalCreate_PayDownRejectsCategoryAndMerchant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filters models.GoalFilters
	}{
		{"category", models.GoalFilters{AccountID: "acc-card", PFCPrimary: "LOAN_PAYMENTS"}},
		{"merchant", models.GoalFilters{AccountID: "acc-card", Merchant: "Chase"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})
			def := validPayDownDef()
			def.Filters = tc.filters
			if _, err := svc.Create(context.Background(), "uid1", "s", def); !isValidationError(err) {
				t.Fatalf("expected ValidationError for %s filter, got %v", tc.name, err)
			}
		})
	}
}

// --- Update ---

func TestGoalUpdate_PartialMerge(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

	_, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		Status: helpers.Ptr(models.GoalStatusCompleted),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for manual completed, got %v", err)
	}
}

func TestGoalUpdate_ReductionPercentRederivesTarget(t *testing.T) {
	goals := newFakeGoalStore()
	seedReductionGoal(goals)
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 20000, Currency: "USD"}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics, &fakeGoalAccounts{})

	updated, err := svc.Update(context.Background(), "uid1", "r1", dto.GoalUpdate{
		ReductionPercent: helpers.Ptr(25.0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Baseline re-measured (20000) × (1 − 25%) = 15000.
	if updated.TargetValueMinor != 15000 {
		t.Fatalf("expected target re-derived to 15000, got %d", updated.TargetValueMinor)
	}
	if updated.BaselineValueMinor == nil || *updated.BaselineValueMinor != 20000 {
		t.Fatalf("expected baseline refreshed to 20000, got %v", updated.BaselineValueMinor)
	}
	if len(analytics.calls) != 1 {
		t.Fatalf("expected the baseline re-measured once, got %d calls", len(analytics.calls))
	}
}

func TestGoalUpdate_ReductionFiltersRemeasureOriginalPeriod(t *testing.T) {
	goals := newFakeGoalStore()
	seedReductionGoal(goals)
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 30000, Currency: "USD"}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics, &fakeGoalAccounts{})

	updated, err := svc.Update(context.Background(), "uid1", "r1", dto.GoalUpdate{
		Filters: &models.GoalFilters{PFCPrimary: "GENERAL_MERCHANDISE"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// New scope baseline 30000 × (1 − 10%) = 27000.
	if updated.TargetValueMinor != 27000 {
		t.Fatalf("expected target re-derived to 27000, got %d", updated.TargetValueMinor)
	}
	if len(analytics.calls) != 1 || helpers.Value(analytics.calls[0].PFCPrimary) != "GENERAL_MERCHANDISE" {
		t.Fatalf("expected baseline re-measured for the new scope, got %+v", analytics.calls)
	}
	// The reference period stays the goal's original creation month (July).
	if helpers.Value(analytics.calls[0].DateFrom) != "2026-07-01" || helpers.Value(analytics.calls[0].DateTo) != "2026-07-31" {
		t.Fatalf("expected the original creation period re-measured, got from=%q to=%q", helpers.Value(analytics.calls[0].DateFrom), helpers.Value(analytics.calls[0].DateTo))
	}
}

func TestGoalUpdate_ReductionTargetValueRejected(t *testing.T) {
	goals := newFakeGoalStore()
	seedReductionGoal(goals)
	analytics := &fakeEvalAnalytics{}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics, &fakeGoalAccounts{})

	_, err := svc.Update(context.Background(), "uid1", "r1", dto.GoalUpdate{
		TargetValueMinor: helpers.Ptr(int64(12345)),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError setting target on a reduction goal, got %v", err)
	}
	if len(analytics.calls) != 0 {
		t.Fatalf("expected no baseline measurement on a rejected update, got %d", len(analytics.calls))
	}
}

func TestGoalUpdate_ReductionCosmeticDoesNotRemeasure(t *testing.T) {
	goals := newFakeGoalStore()
	g := seedReductionGoal(goals)
	originalTarget := g.TargetValueMinor
	analytics := &fakeEvalAnalytics{result: dto.AnalyticsSpendTotalResult{TotalMinor: 99999}}
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, &fakeTransactionsLister{}, analytics, &fakeGoalAccounts{})

	updated, err := svc.Update(context.Background(), "uid1", "r1", dto.GoalUpdate{
		Name: helpers.Ptr("Dining — trimmed"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.TargetValueMinor != originalTarget {
		t.Fatalf("expected target unchanged on a name-only update, got %d", updated.TargetValueMinor)
	}
	if len(analytics.calls) != 0 {
		t.Fatalf("expected no baseline re-measurement on a cosmetic update, got %d", len(analytics.calls))
	}
}

// --- Delete ---

func TestGoalDelete_SubmitsJob(t *testing.T) {
	jobs := &fakeJobs{jobID: "job-xyz"}
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, jobs, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, snaps, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{latest: nil}, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, snaps, &fakeJobs{}, &fakeTransactionsLister{}, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, tx, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

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
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{}, tx, &fakeEvalAnalytics{}, &fakeGoalAccounts{})

	if _, err := svc.ListGoalTransactions(goalContextAt(time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)), "uid1", "g1", nil, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if helpers.Value(tx.args.DateFrom) != "2026-06-01" || helpers.Value(tx.args.DateTo) != "2026-06-30" {
		t.Fatalf("expected window capped at endDate, got from=%q to=%q", helpers.Value(tx.args.DateFrom), helpers.Value(tx.args.DateTo))
	}
}
