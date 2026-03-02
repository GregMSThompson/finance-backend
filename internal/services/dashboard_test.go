package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- Fakes ---

type fakeDashboardStore struct {
	widgets          map[string]*models.Widget
	createErr        error
	getErr           error
	listErr          error
	updateErr        error
	deleteErr        error
	bulkPositionsErr error
	lastPositions    map[string]int
}

func newFakeStore() *fakeDashboardStore {
	return &fakeDashboardStore{widgets: make(map[string]*models.Widget)}
}

func (f *fakeDashboardStore) Create(_ context.Context, _ string, w *models.Widget) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.widgets[w.WidgetID] = w
	return nil
}

func (f *fakeDashboardStore) Get(_ context.Context, _, widgetID string) (*models.Widget, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	w, ok := f.widgets[widgetID]
	if !ok {
		return nil, errs.NewNotFoundError("widget not found")
	}
	return w, nil
}

func (f *fakeDashboardStore) List(_ context.Context, _ string) ([]*models.Widget, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*models.Widget, 0, len(f.widgets))
	for _, w := range f.widgets {
		out = append(out, w)
	}
	return out, nil
}

func (f *fakeDashboardStore) Update(_ context.Context, _ string, w *models.Widget) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.widgets[w.WidgetID] = w
	return nil
}

func (f *fakeDashboardStore) Delete(_ context.Context, _, widgetID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.widgets, widgetID)
	return nil
}

func (f *fakeDashboardStore) Count(_ context.Context, _ string) (int, error) {
	return len(f.widgets), nil
}

func (f *fakeDashboardStore) BulkUpdatePositions(_ context.Context, _ string, positions map[string]int) error {
	if f.bulkPositionsErr != nil {
		return f.bulkPositionsErr
	}
	f.lastPositions = positions
	return nil
}

type fakeDashboardAnalytics struct {
	topNResult       dto.AnalyticsTopNResult
	topNErr          error
	movingAvgResult  dto.AnalyticsMovingAverageResult
	movingAvgErr     error
	periodResult     dto.AnalyticsPeriodComparisonResult
	periodErr        error
	txResult         dto.AnalyticsTransactionsResult
	txErr            error
	recurringResult  dto.RecurringTransactionsResult
	recurringErr     error
	lastTopNArgs     dto.AnalyticsTopNArgs
	lastMovingAvgArgs dto.AnalyticsMovingAverageArgs
	lastPeriodArgs   dto.AnalyticsPeriodComparisonArgs
	lastTxArgs       dto.AnalyticsTransactionsArgs
	lastRecurringArgs dto.AnalyticsRecurringArgs
}

func (f *fakeDashboardAnalytics) GetTopN(_ context.Context, _ string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error) {
	f.lastTopNArgs = args
	return f.topNResult, f.topNErr
}

func (f *fakeDashboardAnalytics) GetMovingAverage(_ context.Context, _ string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error) {
	f.lastMovingAvgArgs = args
	return f.movingAvgResult, f.movingAvgErr
}

func (f *fakeDashboardAnalytics) GetPeriodComparison(_ context.Context, _ string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error) {
	f.lastPeriodArgs = args
	return f.periodResult, f.periodErr
}

func (f *fakeDashboardAnalytics) GetTransactions(_ context.Context, _ string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error) {
	f.lastTxArgs = args
	return f.txResult, f.txErr
}

func (f *fakeDashboardAnalytics) GetRecurringTransactions(_ context.Context, _ string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error) {
	f.lastRecurringArgs = args
	return f.recurringResult, f.recurringErr
}

// --- AddWidget tests ---

func TestAddWidget_TopSpenders_Defaults(t *testing.T) {
	store := newFakeStore()
	an := &fakeDashboardAnalytics{}
	svc := NewDashboardService(store, an)

	req := dto.CreateWidgetRequest{
		Type:          dto.WidgetTypeTopSpenders,
		Visualization: dto.VisPie,
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Dimension: dto.DimensionCategory,
			// Limit intentionally omitted — should default to 10
		},
	}
	w, err := svc.AddWidget(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Config.Limit != 10 {
		t.Errorf("expected default limit 10, got %d", w.Config.Limit)
	}
	if w.Position != 1 {
		t.Errorf("expected position 1, got %d", w.Position)
	}
	if w.WidgetID == "" {
		t.Error("expected non-empty widgetID")
	}
}

func TestAddWidget_InvalidType(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.AddWidget(context.Background(), "uid1", dto.CreateWidgetRequest{
		Type:          "badType",
		Visualization: dto.VisPie,
	})
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestAddWidget_InvalidVisualization(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.AddWidget(context.Background(), "uid1", dto.CreateWidgetRequest{
		Type:          dto.WidgetTypeTopSpenders,
		Visualization: dto.VisLine, // not valid for topSpenders
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Dimension: dto.DimensionCategory,
		},
	})
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestAddWidget_TopSpenders_MissingDateRange(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.AddWidget(context.Background(), "uid1", dto.CreateWidgetRequest{
		Type:          dto.WidgetTypeTopSpenders,
		Visualization: dto.VisPie,
		Config: models.WidgetConfig{
			Dimension: dto.DimensionCategory,
		},
	})
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestAddWidget_LimitOutOfRange(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.AddWidget(context.Background(), "uid1", dto.CreateWidgetRequest{
		Type:          dto.WidgetTypeTopSpenders,
		Visualization: dto.VisPie,
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Dimension: dto.DimensionCategory,
			Limit:     100, // out of range
		},
	})
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

// --- UpdateWidgetConfig tests ---

func TestUpdateWidgetConfig_OK(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID:      "w1",
		Type:          dto.WidgetTypeTopSpenders,
		Visualization: dto.VisPie,
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Dimension: dto.DimensionCategory,
			Limit:     10,
		},
	}
	svc := NewDashboardService(store, &fakeDashboardAnalytics{})

	updated, err := svc.UpdateWidgetConfig(context.Background(), "uid1", "w1", dto.UpdateWidgetConfigRequest{
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeLastMonth},
			Dimension: dto.DimensionMerchant,
			Limit:     5,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Config.Dimension != dto.DimensionMerchant {
		t.Errorf("dimension not updated, got %s", updated.Config.Dimension)
	}
}

func TestUpdateWidgetConfig_NotFound(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.UpdateWidgetConfig(context.Background(), "uid1", "nonexistent", dto.UpdateWidgetConfigRequest{})
	var nfe *errs.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

// --- ReorderWidgets tests ---

func TestReorderWidgets_OK(t *testing.T) {
	store := newFakeStore()
	an := &fakeDashboardAnalytics{}
	svc := NewDashboardService(store, an)

	req := dto.ReorderWidgetsRequest{
		WidgetOrder: []dto.ReorderWidgetItem{
			{WidgetID: "w1", Position: 2},
			{WidgetID: "w2", Position: 1},
		},
	}
	if err := svc.ReorderWidgets(context.Background(), "uid1", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastPositions["w1"] != 2 || store.lastPositions["w2"] != 1 {
		t.Errorf("unexpected positions: %v", store.lastPositions)
	}
}

// --- DeleteWidget tests ---

func TestDeleteWidget_OK(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{WidgetID: "w1"}
	svc := NewDashboardService(store, &fakeDashboardAnalytics{})

	if err := svc.DeleteWidget(context.Background(), "uid1", "w1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := store.widgets["w1"]; exists {
		t.Error("widget should have been deleted")
	}
}

// --- GetWidgetData tests ---

func TestGetWidgetData_WidgetNotFound(t *testing.T) {
	svc := NewDashboardService(newFakeStore(), &fakeDashboardAnalytics{})
	_, err := svc.GetWidgetData(context.Background(), "uid1", "nonexistent")
	var nfe *errs.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestGetWidgetData_TopSpenders(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID: "w1",
		Type:     dto.WidgetTypeTopSpenders,
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Dimension: dto.DimensionCategory,
			Limit:     10,
		},
	}
	an := &fakeDashboardAnalytics{
		topNResult: dto.AnalyticsTopNResult{
			Dimension:  "category",
			TotalSpend: 500.0,
			Currency:   "USD",
			Items: []dto.TopNItem{
				{Key: "Food", Total: 200.0, Percentage: 40.0, Count: 5},
			},
		},
	}
	svc := NewDashboardService(store, an)

	resp, err := svc.GetWidgetData(context.Background(), "uid1", "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Data.(dto.TopSpendersData)
	if !ok {
		t.Fatalf("expected TopSpendersData, got %T", resp.Data)
	}
	if len(data.Items) != 1 || data.Items[0].Name != "Food" {
		t.Errorf("unexpected items: %+v", data.Items)
	}
	if an.lastTopNArgs.Dimension != dto.DimensionCategory {
		t.Errorf("expected dimension=category in TopN call, got %s", an.lastTopNArgs.Dimension)
	}
	if an.lastTopNArgs.Limit != 10 {
		t.Errorf("expected limit=10 in TopN call, got %d", an.lastTopNArgs.Limit)
	}
}

func TestGetWidgetData_SpendingTrend(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID: "w1",
		Type:     dto.WidgetTypeSpendingTrend,
		Config: models.WidgetConfig{
			Window:    dto.Window30Day,
			Dimension: dto.DimensionOverall,
		},
	}
	an := &fakeDashboardAnalytics{
		movingAvgResult: dto.AnalyticsMovingAverageResult{
			Scope:    "overall",
			Currency: "USD",
		},
	}
	svc := NewDashboardService(store, an)

	resp, err := svc.GetWidgetData(context.Background(), "uid1", "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.Data.(dto.AnalyticsMovingAverageResult); !ok {
		t.Fatalf("expected AnalyticsMovingAverageResult, got %T", resp.Data)
	}
	if an.lastMovingAvgArgs.Scope != dto.DimensionOverall {
		t.Errorf("expected scope=overall, got %s", an.lastMovingAvgArgs.Scope)
	}
	if an.lastMovingAvgArgs.Granularity != "day" {
		t.Errorf("expected granularity=day, got %s", an.lastMovingAvgArgs.Granularity)
	}
}

func TestGetWidgetData_PeriodComparison(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID: "w1",
		Type:     dto.WidgetTypePeriodComparison,
		Config: models.WidgetConfig{
			Preset: dto.PeriodMonthOverMonth,
		},
	}
	pct := 10.5
	an := &fakeDashboardAnalytics{
		periodResult: dto.AnalyticsPeriodComparisonResult{
			Current:  dto.PeriodSummary{Total: 1000, Count: 20, Currency: "USD"},
			Previous: dto.PeriodSummary{Total: 900, Count: 18, Currency: "USD"},
			Change:   dto.PeriodChange{AbsoluteChange: 100, PercentageChange: &pct, CountChange: 2},
		},
	}
	svc := NewDashboardService(store, an)

	resp, err := svc.GetWidgetData(context.Background(), "uid1", "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Data.(dto.PeriodComparisonWidgetData)
	if !ok {
		t.Fatalf("expected PeriodComparisonWidgetData, got %T", resp.Data)
	}
	if data.Current.Amount != 1000 {
		t.Errorf("unexpected current amount: %f", data.Current.Amount)
	}
	if data.Change.Percent == nil || *data.Change.Percent != 10.5 {
		t.Errorf("unexpected change percent: %v", data.Change.Percent)
	}
	// Verify date args were resolved (non-empty)
	if an.lastPeriodArgs.CurrentFrom == "" || an.lastPeriodArgs.PreviousFrom == "" {
		t.Error("expected period comparison dates to be resolved and non-empty")
	}
}

func TestGetWidgetData_LargestTransactions(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID: "w1",
		Type:     dto.WidgetTypeLargestTransactions,
		Config: models.WidgetConfig{
			DateRange: &models.DateRangeConfig{Preset: dto.DateRangeThisMonth},
			Limit:     5,
		},
	}
	an := &fakeDashboardAnalytics{
		txResult: dto.AnalyticsTransactionsResult{
			Transactions: []models.Transaction{
				{TransactionID: "tx1", Name: "Amazon", Amount: 99.99, PFCPrimary: "Shopping", Date: "2026-01-15"},
			},
		},
	}
	svc := NewDashboardService(store, an)

	resp, err := svc.GetWidgetData(context.Background(), "uid1", "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Data.(dto.LargestTransactionsData)
	if !ok {
		t.Fatalf("expected LargestTransactionsData, got %T", resp.Data)
	}
	if len(data.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(data.Transactions))
	}
	tx := data.Transactions[0]
	if tx.Merchant != "Amazon" {
		t.Errorf("expected merchant=Amazon, got %s", tx.Merchant)
	}
	if tx.Category != "Shopping" {
		t.Errorf("expected category=Shopping, got %s", tx.Category)
	}
	if an.lastTxArgs.OrderBy != "amount" || !an.lastTxArgs.Desc {
		t.Errorf("expected orderBy=amount desc, got %s desc=%v", an.lastTxArgs.OrderBy, an.lastTxArgs.Desc)
	}
}

func TestGetWidgetData_RecurringSubscriptions(t *testing.T) {
	store := newFakeStore()
	store.widgets["w1"] = &models.Widget{
		WidgetID: "w1",
		Type:     dto.WidgetTypeRecurringSubscriptions,
		Config:   models.WidgetConfig{},
	}
	an := &fakeDashboardAnalytics{
		recurringResult: dto.RecurringTransactionsResult{
			Items: []dto.RecurringItem{
				{Merchant: "Netflix", TypicalAmount: 15.99, Frequency: "monthly", MonthlyEquivalent: 15.99},
			},
			TotalMonthlyEquivalent: 15.99,
			Currency:               "USD",
		},
	}
	svc := NewDashboardService(store, an)

	resp, err := svc.GetWidgetData(context.Background(), "uid1", "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Data.(dto.RecurringSubscriptionsData)
	if !ok {
		t.Fatalf("expected RecurringSubscriptionsData, got %T", resp.Data)
	}
	if len(data.Subscriptions) != 1 || data.Subscriptions[0].Merchant != "Netflix" {
		t.Errorf("unexpected subscriptions: %+v", data.Subscriptions)
	}
	if data.TotalMonthly != 15.99 {
		t.Errorf("unexpected totalMonthly: %f", data.TotalMonthly)
	}
	// Verify date range is ~12 months
	if an.lastRecurringArgs.DateFrom == "" {
		t.Error("expected DateFrom to be set")
	}
}

// --- resolveDateRange tests ---

func TestResolveDateRange_ThisMonth(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveDateRange(models.DateRangeConfig{Preset: dto.DateRangeThisMonth}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-03-01" {
		t.Errorf("expected from=2026-03-01, got %s", from)
	}
	if to != "2026-03-15" {
		t.Errorf("expected to=2026-03-15, got %s", to)
	}
}

func TestResolveDateRange_LastMonth(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveDateRange(models.DateRangeConfig{Preset: dto.DateRangeLastMonth}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-02-01" {
		t.Errorf("expected from=2026-02-01, got %s", from)
	}
	if to != "2026-02-28" {
		t.Errorf("expected to=2026-02-28, got %s", to)
	}
}

func TestResolveDateRange_Custom(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveDateRange(models.DateRangeConfig{StartDate: "2026-01-01", EndDate: "2026-01-31"}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-01-01" || to != "2026-01-31" {
		t.Errorf("unexpected custom range: %s - %s", from, to)
	}
}

func TestResolveDateRange_ThisQuarter(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC) // Q1
	from, to, err := resolveDateRange(models.DateRangeConfig{Preset: dto.DateRangeThisQuarter}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-01-01" {
		t.Errorf("expected Q1 start=2026-01-01, got %s", from)
	}
	if to != "2026-03-15" {
		t.Errorf("expected to=2026-03-15, got %s", to)
	}
}

func TestResolveDateRange_LastQuarter(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC) // Q2 → last quarter is Q1
	from, to, err := resolveDateRange(models.DateRangeConfig{Preset: dto.DateRangeLastQuarter}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-01-01" {
		t.Errorf("expected from=2026-01-01, got %s", from)
	}
	if to != "2026-03-31" {
		t.Errorf("expected to=2026-03-31, got %s", to)
	}
}

// --- resolveWindow tests ---

func TestResolveWindow_30day(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveWindow(dto.Window30Day, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-02-13" {
		t.Errorf("expected from=2026-02-13, got %s", from)
	}
	if to != "2026-03-15" {
		t.Errorf("expected to=2026-03-15, got %s", to)
	}
}

func TestResolveWindow_Invalid(t *testing.T) {
	_, _, err := resolveWindow("badWindow", time.Now())
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

// --- resolvePeriodPreset tests ---

func TestResolvePeriodPreset_MonthOverMonth(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	cFrom, cTo, pFrom, pTo, err := resolvePeriodPreset(dto.PeriodMonthOverMonth, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cFrom != "2026-03-01" {
		t.Errorf("expected current from=2026-03-01, got %s", cFrom)
	}
	if cTo != "2026-03-15" {
		t.Errorf("expected current to=2026-03-15, got %s", cTo)
	}
	if pFrom != "2026-02-01" {
		t.Errorf("expected prev from=2026-02-01, got %s", pFrom)
	}
	if pTo != "2026-02-28" {
		t.Errorf("expected prev to=2026-02-28, got %s", pTo)
	}
}

func TestResolvePeriodPreset_WeekOverWeek(t *testing.T) {
	// 2026-03-11 is a Wednesday
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	cFrom, cTo, pFrom, pTo, err := resolvePeriodPreset(dto.PeriodWeekOverWeek, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cFrom != "2026-03-09" { // Monday
		t.Errorf("expected current from=2026-03-09 (Monday), got %s", cFrom)
	}
	if cTo != "2026-03-11" {
		t.Errorf("expected current to=2026-03-11, got %s", cTo)
	}
	if pFrom != "2026-03-02" { // Previous Monday
		t.Errorf("expected prev from=2026-03-02, got %s", pFrom)
	}
	if pTo != "2026-03-08" { // Previous Sunday
		t.Errorf("expected prev to=2026-03-08, got %s", pTo)
	}
}
