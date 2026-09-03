package services

import (
	"context"
	"errors"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type fakeAnalyticsStore struct {
	txs       []*models.Transaction
	err       error
	lastUID   string
	lastQuery dto.TransactionQuery
}

func (f *fakeAnalyticsStore) Query(ctx context.Context, uid string, q dto.TransactionQuery, handle func(*models.Transaction) error) error {
	f.lastUID = uid
	f.lastQuery = q
	for _, tx := range f.txs {
		if err := handle(tx); err != nil {
			return err
		}
	}
	return f.err
}

func TestAnalyticsSpendTotal(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 1050, Currency: "USD"},
			{AmountMinor: 225, Currency: "USD"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendTotal(context.Background(), "user", dto.AnalyticsSpendTotalArgs{})
	if err != nil {
		t.Fatalf("GetSpendTotal error: %v", err)
	}

	if got.TotalMinor != 1275 {
		t.Fatalf("total mismatch: got %v", got.TotalMinor)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
}

func TestAnalyticsSpendTotalExcludesIncomeAndTransfers(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 4000, Currency: "USD", PFCPrimary: "FOOD_AND_DRINK"},
			// Inflows carry Plaid's negative sign; they're excluded regardless.
			{AmountMinor: -500000, Currency: "USD", PFCPrimary: "INCOME"},
			{AmountMinor: -30000, Currency: "USD", PFCPrimary: "TRANSFER_IN"},
			{AmountMinor: 20000, Currency: "USD", PFCPrimary: "TRANSFER_OUT"},
			{AmountMinor: 1000, Currency: "USD", PFCPrimary: "ENTERTAINMENT"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendTotal(context.Background(), "user", dto.AnalyticsSpendTotalArgs{})
	if err != nil {
		t.Fatalf("GetSpendTotal error: %v", err)
	}
	if got.TotalMinor != 5000 {
		t.Fatalf("expected only spend categories summed (5000), got %v", got.TotalMinor)
	}
}

func TestAnalyticsSpendTotalHonorsExplicitIncomeFilter(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: -500000, Currency: "USD", PFCPrimary: "INCOME"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendTotal(context.Background(), "user", dto.AnalyticsSpendTotalArgs{
		PFCPrimary: helpers.Ptr("INCOME"),
	})
	if err != nil {
		t.Fatalf("GetSpendTotal error: %v", err)
	}
	// An explicit filter returns the category's raw signed sum — income keeps its
	// inflow-negative sign here. Normalizing it to a positive magnitude is the job
	// of a dedicated income accessor, not GetSpendTotal.
	if got.TotalMinor != -500000 {
		t.Fatalf("explicit INCOME filter should return the raw income sum (-500000), got %v", got.TotalMinor)
	}
}

func TestAnalyticsSpendBreakdown(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Coffee", AmountMinor: 300, Currency: "USD"},
			{Name: "Coffee", AmountMinor: 200, Currency: "USD"},
			{Name: "Lunch", AmountMinor: 800, Currency: "USD"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendBreakdown(context.Background(), "user", dto.AnalyticsSpendBreakdownArgs{
		GroupBy: "merchant",
	})
	if err != nil {
		t.Fatalf("GetSpendBreakdown error: %v", err)
	}

	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length mismatch: got %d", len(got.Items))
	}

	items := map[string]dto.AnalyticsBreakdownItem{}
	for _, item := range got.Items {
		items[item.Key] = item
	}
	if items["Coffee"].TotalMinor != 500 || items["Coffee"].Count != 2 {
		t.Fatalf("coffee totals mismatch: %+v", items["Coffee"])
	}
	if items["Lunch"].TotalMinor != 800 || items["Lunch"].Count != 1 {
		t.Fatalf("lunch totals mismatch: %+v", items["Lunch"])
	}
}

func TestAnalyticsSpendBreakdownInvalidGroupBy(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	_, err := svc.GetSpendBreakdown(context.Background(), "user", dto.AnalyticsSpendBreakdownArgs{
		GroupBy: "unknown",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var groupErr *errs.UnsupportedGroupByError
	if !errors.As(err, &groupErr) {
		t.Fatalf("expected UnsupportedGroupByError, got %T", err)
	}
}

func TestAnalyticsSpendTotalPropagatesStoreError(t *testing.T) {
	store := &fakeAnalyticsStore{
		err: errors.New("store down"),
	}
	svc := NewAnalyticsService(store)

	_, err := svc.GetSpendTotal(context.Background(), "user", dto.AnalyticsSpendTotalArgs{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAnalyticsSpendTotalPassesFilters(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	merchant := "starbucks"
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.AnalyticsSpendTotalArgs{
		Merchant: &merchant,
		DateFrom: &from,
		DateTo:   &to,
	}

	_, err := svc.GetSpendTotal(context.Background(), "user-123", args)
	if err != nil {
		t.Fatalf("GetSpendTotal error: %v", err)
	}
	if store.lastQuery.Merchant == nil || *store.lastQuery.Merchant != "starbucks" {
		t.Fatalf("merchant mismatch: %+v", store.lastQuery.Merchant)
	}
	if store.lastQuery.DateFrom == nil || *store.lastQuery.DateFrom != "2025-01-01" {
		t.Fatalf("dateFrom mismatch: %+v", store.lastQuery.DateFrom)
	}
	if store.lastQuery.DateTo == nil || *store.lastQuery.DateTo != "2025-01-31" {
		t.Fatalf("dateTo mismatch: %+v", store.lastQuery.DateTo)
	}
}

// funcAnalyticsStore routes each Query call through a user-supplied function,
// allowing tests to return different transactions for different date ranges.
type funcAnalyticsStore struct {
	fn func(q dto.TransactionQuery) ([]*models.Transaction, error)
}

func (f *funcAnalyticsStore) Query(_ context.Context, _ string, q dto.TransactionQuery, handle func(*models.Transaction) error) error {
	txs, err := f.fn(q)
	if err != nil {
		return err
	}
	for _, tx := range txs {
		if err := handle(tx); err != nil {
			return err
		}
	}
	return nil
}

func TestGetPeriodComparisonBasicTotal(t *testing.T) {
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return []*models.Transaction{
					{AmountMinor: 3000, Currency: "USD"},
					{AmountMinor: 2000, Currency: "USD"},
				}, nil
			}
			return []*models.Transaction{
				{AmountMinor: 4000, Currency: "USD"},
			}, nil
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetPeriodComparison error: %v", err)
	}

	if got.Current.TotalMinor != 5000 || got.Current.Count != 2 {
		t.Fatalf("current mismatch: total=%v count=%v", got.Current.TotalMinor, got.Current.Count)
	}
	if got.Previous.TotalMinor != 4000 || got.Previous.Count != 1 {
		t.Fatalf("previous mismatch: total=%v count=%v", got.Previous.TotalMinor, got.Previous.Count)
	}
	if got.Change.AbsoluteChangeMinor != 1000 {
		t.Fatalf("absolute change mismatch: %v", got.Change.AbsoluteChangeMinor)
	}
	if got.Change.PercentageChange == nil {
		t.Fatal("expected non-nil percentage change")
	}
	if helpers.Value(got.Change.PercentageChange) != 25 {
		t.Fatalf("percentage change mismatch: %v", helpers.Value(got.Change.PercentageChange))
	}
	if got.Change.CountChange != 1 {
		t.Fatalf("count change mismatch: %v", got.Change.CountChange)
	}
	if got.Current.Currency != "USD" {
		t.Fatalf("currency mismatch: %q", got.Current.Currency)
	}
	if got.Change.Currency != "USD" {
		t.Fatalf("change currency mismatch: %q", got.Change.Currency)
	}
	if got.Current.Items != nil || got.Previous.Items != nil || got.Change.Items != nil {
		t.Fatal("expected nil items when no groupBy")
	}
}

func TestGetPeriodComparisonChangeCurrencyCoalescesFromPrevious(t *testing.T) {
	// Current period has no transactions (and so no currency of its own); the
	// change is still meaningful and must carry the previous period's currency.
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return nil, nil
			}
			return []*models.Transaction{{AmountMinor: 4000, Currency: "USD"}}, nil
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetPeriodComparison error: %v", err)
	}

	if got.Current.Currency != "" {
		t.Fatalf("expected empty current currency for an empty period, got %q", got.Current.Currency)
	}
	if got.Change.AbsoluteChangeMinor != -4000 {
		t.Fatalf("absolute change mismatch: %v", got.Change.AbsoluteChangeMinor)
	}
	if got.Change.Currency != "USD" {
		t.Fatalf("expected change currency coalesced to USD, got %q", got.Change.Currency)
	}
}

func TestGetPeriodComparisonNilPercentageWhenPreviousZero(t *testing.T) {
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return []*models.Transaction{
					{AmountMinor: 3000, Currency: "USD"},
				}, nil
			}
			return nil, nil
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetPeriodComparison error: %v", err)
	}

	if got.Change.PercentageChange != nil {
		t.Fatalf("expected nil percentage change when previous=0, got %v", *got.Change.PercentageChange)
	}
	if got.Change.AbsoluteChangeMinor != 3000 {
		t.Fatalf("absolute change mismatch: %v", got.Change.AbsoluteChangeMinor)
	}
}

func TestGetPeriodComparisonWithGroupBy(t *testing.T) {
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return []*models.Transaction{
					{Name: "Coffee", AmountMinor: 500, Currency: "USD"},
					{Name: "Lunch", AmountMinor: 1000, Currency: "USD"},
				}, nil
			}
			return []*models.Transaction{
				{Name: "Coffee", AmountMinor: 400, Currency: "USD"},
				{Name: "Dinner", AmountMinor: 800, Currency: "USD"},
			}, nil
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
		GroupBy:      "merchant",
	})
	if err != nil {
		t.Fatalf("GetPeriodComparison error: %v", err)
	}

	if len(got.Current.Items) != 2 {
		t.Fatalf("current items length mismatch: %d", len(got.Current.Items))
	}
	if len(got.Previous.Items) != 2 {
		t.Fatalf("previous items length mismatch: %d", len(got.Previous.Items))
	}
	// Union of Coffee, Lunch, Dinner = 3 change items
	if len(got.Change.Items) != 3 {
		t.Fatalf("change items length mismatch: %d", len(got.Change.Items))
	}

	changeByKey := map[string]dto.BreakdownItemChange{}
	for _, item := range got.Change.Items {
		changeByKey[item.Key] = item
	}

	coffee := changeByKey["Coffee"]
	if coffee.AbsoluteChangeMinor != 100 {
		t.Fatalf("Coffee absolute change mismatch: %v", coffee.AbsoluteChangeMinor)
	}
	if coffee.PercentageChange == nil || helpers.Value(coffee.PercentageChange) != 25 {
		t.Fatalf("Coffee percentage change mismatch: %v", coffee.PercentageChange)
	}

	lunch := changeByKey["Lunch"]
	if lunch.AbsoluteChangeMinor != 1000 {
		t.Fatalf("Lunch absolute change mismatch: %v", lunch.AbsoluteChangeMinor)
	}
	if lunch.PercentageChange != nil {
		t.Fatalf("Lunch expected nil percentage (previous=0), got %v", *lunch.PercentageChange)
	}

	dinner := changeByKey["Dinner"]
	if dinner.AbsoluteChangeMinor != -800 {
		t.Fatalf("Dinner absolute change mismatch: %v", dinner.AbsoluteChangeMinor)
	}
}

func TestGetPeriodComparisonInvalidGroupBy(t *testing.T) {
	store := &funcAnalyticsStore{fn: func(_ dto.TransactionQuery) ([]*models.Transaction, error) { return nil, nil }}
	svc := NewAnalyticsService(store)

	_, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
		GroupBy:      "unknown",
	})
	if err == nil {
		t.Fatal("expected error for invalid groupBy")
	}
	var groupErr *errs.UnsupportedGroupByError
	if !errors.As(err, &groupErr) {
		t.Fatalf("expected UnsupportedGroupByError, got %T", err)
	}
}

func TestGetPeriodComparisonStoreErrorPropagates(t *testing.T) {
	storeErr := errors.New("store down")
	store := &funcAnalyticsStore{
		fn: func(_ dto.TransactionQuery) ([]*models.Transaction, error) {
			return nil, storeErr
		},
	}
	svc := NewAnalyticsService(store)

	_, err := svc.GetPeriodComparison(context.Background(), "user", dto.AnalyticsPeriodComparisonArgs{
		CurrentFrom:  "2025-02-01",
		CurrentTo:    "2025-02-28",
		PreviousFrom: "2025-01-01",
		PreviousTo:   "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestGetRecurringTransactionsMonthly(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Netflix", AmountMinor: 1599, Currency: "USD", Date: "2025-01-15"},
			{Name: "Netflix", AmountMinor: 1599, Currency: "USD", Date: "2025-02-14"},
			{Name: "Netflix", AmountMinor: 1599, Currency: "USD", Date: "2025-03-15"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetRecurringTransactions error: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Items))
	}
	item := got.Items[0]
	if item.Merchant != "Netflix" {
		t.Fatalf("merchant mismatch: %q", item.Merchant)
	}
	if item.Frequency != "monthly" {
		t.Fatalf("frequency mismatch: %q", item.Frequency)
	}
	if item.TypicalAmountMinor != 1599 {
		t.Fatalf("typical amount mismatch: %v", item.TypicalAmountMinor)
	}
	if item.AmountIsVariable {
		t.Fatal("expected amount not variable")
	}
	if item.OccurrenceCount != 3 {
		t.Fatalf("occurrence count mismatch: %d", item.OccurrenceCount)
	}
	if item.LastDate != "2025-03-15" {
		t.Fatalf("last date mismatch: %q", item.LastDate)
	}
	if item.MonthlyEquivalentMinor != 1599 {
		t.Fatalf("monthly equivalent mismatch: %v", item.MonthlyEquivalentMinor)
	}
	if got.TotalMonthlyEquivalentMinor != 1599 {
		t.Fatalf("total monthly equivalent mismatch: %v", got.TotalMonthlyEquivalentMinor)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: %q", got.Currency)
	}
}

func TestGetRecurringTransactionsWeekly(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Gym", AmountMinor: 1000, Currency: "USD", Date: "2025-01-06"},
			{Name: "Gym", AmountMinor: 1000, Currency: "USD", Date: "2025-01-13"},
			{Name: "Gym", AmountMinor: 1000, Currency: "USD", Date: "2025-01-20"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetRecurringTransactions error: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Items))
	}
	if got.Items[0].Frequency != "weekly" {
		t.Fatalf("frequency mismatch: %q", got.Items[0].Frequency)
	}
	// 1000 * 4.33 = 4330 (minor units)
	if got.TotalMonthlyEquivalentMinor != 4330 {
		t.Fatalf("total monthly equivalent mismatch: %v", got.TotalMonthlyEquivalentMinor)
	}
}

func TestGetRecurringTransactionsDropsInsufficientOccurrences(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "One-off", AmountMinor: 5000, Currency: "USD", Date: "2025-01-10"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetRecurringTransactions error: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(got.Items))
	}
}

func TestGetRecurringTransactionsDropsIrregular(t *testing.T) {
	// gaps: 1, 18, 30 days → sorted median = 18 is biweekly, but let's use
	// gaps that don't fit any bucket: 60 days between two transactions.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Irregular", AmountMinor: 2000, Currency: "USD", Date: "2025-01-01"},
			{Name: "Irregular", AmountMinor: 2000, Currency: "USD", Date: "2025-03-02"}, // 60 day gap
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetRecurringTransactions error: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("expected 0 items for irregular gaps, got %d", len(got.Items))
	}
}

func TestGetRecurringTransactionsVariableAmount(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Utility", AmountMinor: 8000, Currency: "USD", Date: "2025-01-15"},
			{Name: "Utility", AmountMinor: 11000, Currency: "USD", Date: "2025-02-15"},
			{Name: "Utility", AmountMinor: 9500, Currency: "USD", Date: "2025-03-15"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetRecurringTransactions error: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Items))
	}
	if !got.Items[0].AmountIsVariable {
		t.Fatal("expected AmountIsVariable=true")
	}
}

func TestGetRecurringTransactionsStoreErrorPropagates(t *testing.T) {
	store := &fakeAnalyticsStore{err: errors.New("store down")}
	svc := NewAnalyticsService(store)

	_, err := svc.GetRecurringTransactions(context.Background(), "user", dto.AnalyticsRecurringArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-03-31",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestGetMovingAverageOverallMonthly(t *testing.T) {
	// 3 transactions across 3 months; 2025-01-01 to 2025-03-31 = 90 days → 3 monthly units.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 10000, Currency: "USD", Date: "2025-01-15"},
			{AmountMinor: 8000, Currency: "USD", Date: "2025-02-15"},
			{AmountMinor: 12000, Currency: "USD", Date: "2025-03-15"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "month",
		Scope:       "overall",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-03-31",
	})
	if err != nil {
		t.Fatalf("GetMovingAverage error: %v", err)
	}
	if got.DaysAnalyzed != 90 {
		t.Fatalf("daysAnalyzed mismatch: got %d", got.DaysAnalyzed)
	}
	if got.TransactionCount != 3 {
		t.Fatalf("transactionCount mismatch: got %d", got.TransactionCount)
	}
	// total=30000, units=90/30=3 → average=10000
	if got.AveragePerUnitMinor != 10000 {
		t.Fatalf("averagePerUnit mismatch: got %v", got.AveragePerUnitMinor)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
	if len(got.Series) != 3 {
		t.Fatalf("series length mismatch: got %d", len(got.Series))
	}
	// Series must be sorted by period key.
	if got.Series[0].Period != "2025-01" || got.Series[1].Period != "2025-02" || got.Series[2].Period != "2025-03" {
		t.Fatalf("series periods mismatch: %+v", got.Series)
	}
	if got.Series[0].TotalMinor != 10000 || got.Series[1].TotalMinor != 8000 || got.Series[2].TotalMinor != 12000 {
		t.Fatalf("series totals mismatch: %+v", got.Series)
	}
	if got.Items != nil {
		t.Fatal("expected nil items for scope=overall")
	}
}

func TestGetMovingAverageWeeklyPeriodKeys(t *testing.T) {
	// 2025-01-06 = ISO week 2025-W02, 2025-01-13 = ISO week 2025-W03.
	// Window: Jan 6–19 = 14 days → 2 weekly units.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 1000, Currency: "USD", Date: "2025-01-06"},
			{AmountMinor: 2000, Currency: "USD", Date: "2025-01-13"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "week",
		Scope:       "overall",
		DateFrom:    "2025-01-06",
		DateTo:      "2025-01-19",
	})
	if err != nil {
		t.Fatalf("GetMovingAverage error: %v", err)
	}
	if got.DaysAnalyzed != 14 {
		t.Fatalf("daysAnalyzed mismatch: got %d", got.DaysAnalyzed)
	}
	if len(got.Series) != 2 {
		t.Fatalf("series length mismatch: got %d", len(got.Series))
	}
	if got.Series[0].Period != "2025-W02" || got.Series[1].Period != "2025-W03" {
		t.Fatalf("week period keys mismatch: %+v", got.Series)
	}
	// total=3000, units=14/7=2 → average=1500
	if got.AveragePerUnitMinor != 1500 {
		t.Fatalf("averagePerUnit mismatch: got %v", got.AveragePerUnitMinor)
	}
}

func TestGetMovingAverageScopeCategory(t *testing.T) {
	// 2 categories across a 31-day window.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 5000, Currency: "USD", Date: "2025-01-10", PFCPrimary: "Food"},
			{AmountMinor: 3000, Currency: "USD", Date: "2025-01-15", PFCPrimary: "Food"},
			{AmountMinor: 2000, Currency: "USD", Date: "2025-01-20", PFCPrimary: "Transport"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "month",
		Scope:       "category",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetMovingAverage error: %v", err)
	}
	if got.TransactionCount != 3 {
		t.Fatalf("transactionCount mismatch: got %d", got.TransactionCount)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length mismatch: got %d", len(got.Items))
	}
	// Items are sorted by key: Food < Transport.
	if got.Items[0].Key != "Food" || got.Items[1].Key != "Transport" {
		t.Fatalf("items key order mismatch: %+v", got.Items)
	}
	if got.Items[0].TransactionCount != 2 || got.Items[1].TransactionCount != 1 {
		t.Fatalf("items transaction counts mismatch")
	}
	// Each item must have its own series.
	if len(got.Items[0].Series) != 1 || got.Items[0].Series[0].Period != "2025-01" {
		t.Fatalf("food series mismatch: %+v", got.Items[0].Series)
	}
	if len(got.Items[1].Series) != 1 || got.Items[1].Series[0].Period != "2025-01" {
		t.Fatalf("transport series mismatch: %+v", got.Items[1].Series)
	}
	// Overall series still present.
	if len(got.Series) != 1 {
		t.Fatalf("overall series length mismatch: got %d", len(got.Series))
	}
}

func TestGetMovingAverageNoTransactions(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	got, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "month",
		Scope:       "overall",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetMovingAverage error: %v", err)
	}
	if got.TransactionCount != 0 {
		t.Fatalf("expected transactionCount=0, got %d", got.TransactionCount)
	}
	if got.AveragePerUnitMinor != 0 {
		t.Fatalf("expected averagePerUnit=0, got %v", got.AveragePerUnitMinor)
	}
}

func TestGetMovingAverageInvalidGranularity(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	_, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "year",
		Scope:       "overall",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error for invalid granularity")
	}
	var valErr *errs.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestGetMovingAverageInvalidScope(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	_, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "month",
		Scope:       "unknown",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	var valErr *errs.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestGetMovingAverageStoreErrorPropagates(t *testing.T) {
	store := &fakeAnalyticsStore{err: errors.New("store down")}
	svc := NewAnalyticsService(store)

	_, err := svc.GetMovingAverage(context.Background(), "user", dto.AnalyticsMovingAverageArgs{
		Granularity: "month",
		Scope:       "overall",
		DateFrom:    "2025-01-01",
		DateTo:      "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestGetTopNByMerchantTop(t *testing.T) {
	// Alpha=6000, Beta=3000, Gamma=1000, total=10000. Top 2: Alpha then Beta.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Alpha", AmountMinor: 6000, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", AmountMinor: 3000, Currency: "USD", Date: "2025-01-11"},
			{Name: "Gamma", AmountMinor: 1000, Currency: "USD", Date: "2025-01-12"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "merchant",
		Direction: "top",
		Limit:     2,
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetTopN error: %v", err)
	}
	if got.TotalSpendMinor != 10000 {
		t.Fatalf("totalSpend mismatch: got %v", got.TotalSpendMinor)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length mismatch: got %d", len(got.Items))
	}
	if got.Items[0].Key != "Alpha" || got.Items[1].Key != "Beta" {
		t.Fatalf("item order mismatch: %+v", got.Items)
	}
	if got.Items[0].TotalMinor != 6000 || got.Items[1].TotalMinor != 3000 {
		t.Fatalf("item totals mismatch: %+v", got.Items)
	}
	if got.Items[0].Percentage != 60 || got.Items[1].Percentage != 30 {
		t.Fatalf("item percentages mismatch: %+v", got.Items)
	}
}

func TestGetTopNByCategoryReturnsAll(t *testing.T) {
	// 2 categories, limit=5 → returns both.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{PFCPrimary: "Food", AmountMinor: 8000, Currency: "USD", Date: "2025-01-10"},
			{PFCPrimary: "Food", AmountMinor: 2000, Currency: "USD", Date: "2025-01-15"},
			{PFCPrimary: "Transport", AmountMinor: 5000, Currency: "USD", Date: "2025-01-12"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "category",
		Direction: "top",
		Limit:     5,
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetTopN error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.Items))
	}
	// Food=10000, Transport=5000 → top order.
	if got.Items[0].Key != "Food" || got.Items[1].Key != "Transport" {
		t.Fatalf("item order mismatch: %+v", got.Items)
	}
	if got.Items[0].Count != 2 || got.Items[1].Count != 1 {
		t.Fatalf("item counts mismatch: %+v", got.Items)
	}
}

func TestGetTopNBottom(t *testing.T) {
	// Alpha=6000, Beta=3000, Gamma=1000, total=10000. Bottom 2: Gamma then Beta.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Alpha", AmountMinor: 6000, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", AmountMinor: 3000, Currency: "USD", Date: "2025-01-11"},
			{Name: "Gamma", AmountMinor: 1000, Currency: "USD", Date: "2025-01-12"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "merchant",
		Direction: "bottom",
		Limit:     2,
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetTopN error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length mismatch: got %d", len(got.Items))
	}
	if got.Items[0].Key != "Gamma" || got.Items[1].Key != "Beta" {
		t.Fatalf("item order mismatch: %+v", got.Items)
	}
}

func TestGetTopNMinCountFilter(t *testing.T) {
	// Alpha has 1 tx, Beta has 2. MinCount=2 → only Beta included.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Alpha", AmountMinor: 6000, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", AmountMinor: 3000, Currency: "USD", Date: "2025-01-11"},
			{Name: "Beta", AmountMinor: 2000, Currency: "USD", Date: "2025-01-15"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "merchant",
		Direction: "top",
		Limit:     5,
		MinCount:  2,
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetTopN error: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item after MinCount filter, got %d", len(got.Items))
	}
	if got.Items[0].Key != "Beta" {
		t.Fatalf("expected Beta, got %q", got.Items[0].Key)
	}
	if got.Items[0].Count != 2 {
		t.Fatalf("expected count=2, got %d", got.Items[0].Count)
	}
}

func TestGetTopNNoTransactions(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "merchant",
		Direction: "top",
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetTopN error: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(got.Items))
	}
	if got.TotalSpendMinor != 0 {
		t.Fatalf("expected totalSpend=0, got %v", got.TotalSpendMinor)
	}
}

func TestGetTopNInvalidDimension(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	_, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "day",
		Direction: "top",
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error for invalid dimension")
	}
	var valErr *errs.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestGetTopNStoreErrorPropagates(t *testing.T) {
	store := &fakeAnalyticsStore{err: errors.New("store down")}
	svc := NewAnalyticsService(store)

	_, err := svc.GetTopN(context.Background(), "user", dto.AnalyticsTopNArgs{
		Dimension: "merchant",
		Direction: "top",
		DateFrom:  "2025-01-01",
		DateTo:    "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestGetIncomeVsExpensesBasic(t *testing.T) {
	// Income arrives with Plaid's negative sign (-500000) but is reported as a
	// positive magnitude: 500000 income, 20000+10000=30000 expenses → net 470000.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: -500000, Currency: "USD", PFCPrimary: "INCOME"},
			{AmountMinor: 20000, Currency: "USD", PFCPrimary: "DINING"},
			{AmountMinor: 10000, Currency: "USD", PFCPrimary: "ENTERTAINMENT"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetIncomeVsExpenses(context.Background(), "user", dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetIncomeVsExpenses error: %v", err)
	}
	if got.IncomeMinor != 500000 {
		t.Fatalf("income mismatch: got %v", got.IncomeMinor)
	}
	if got.ExpensesMinor != 30000 {
		t.Fatalf("expenses mismatch: got %v", got.ExpensesMinor)
	}
	if got.NetMinor != 470000 {
		t.Fatalf("net mismatch: got %v", got.NetMinor)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
	if got.From != "2025-01-01" || got.To != "2025-01-31" {
		t.Fatalf("date range mismatch: from=%q to=%q", got.From, got.To)
	}
}

func TestGetIncomeVsExpensesNormalizesSignAndExcludesTransfers(t *testing.T) {
	// Income inflow is negative; a refund nets its spend category down; both
	// transfer legs are ignored entirely.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: -500000, Currency: "USD", PFCPrimary: "INCOME"},
			{AmountMinor: 20000, Currency: "USD", PFCPrimary: "FOOD_AND_DRINK"},
			{AmountMinor: 5000, Currency: "USD", PFCPrimary: "GENERAL_MERCHANDISE"},
			{AmountMinor: -2000, Currency: "USD", PFCPrimary: "GENERAL_MERCHANDISE"}, // refund
			{AmountMinor: -100000, Currency: "USD", PFCPrimary: "TRANSFER_IN"},
			{AmountMinor: 100000, Currency: "USD", PFCPrimary: "TRANSFER_OUT"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetIncomeVsExpenses(context.Background(), "user", dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: "2026-01-01",
		DateTo:   "2026-01-31",
	})
	if err != nil {
		t.Fatalf("GetIncomeVsExpenses error: %v", err)
	}
	// income 500000; expenses = 20000 + 5000 - 2000 = 23000; net = 477000.
	if got.IncomeMinor != 500000 {
		t.Fatalf("income should be a positive magnitude (500000), got %d", got.IncomeMinor)
	}
	if got.ExpensesMinor != 23000 {
		t.Fatalf("expenses should net the refund and exclude transfers (23000), got %d", got.ExpensesMinor)
	}
	if got.NetMinor != 477000 {
		t.Fatalf("net mismatch: got %d", got.NetMinor)
	}
}

func TestGetSpendBreakdownExcludesIncomeAndTransfers(t *testing.T) {
	// collectPeriod (shared by breakdown, period-comparison, top-N) must drop
	// non-spend categories, so income/transfers never appear as a spend group.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 4000, Currency: "USD", PFCPrimary: "FOOD_AND_DRINK"},
			{AmountMinor: -500000, Currency: "USD", PFCPrimary: "INCOME"},
			{AmountMinor: -30000, Currency: "USD", PFCPrimary: "TRANSFER_IN"},
			{AmountMinor: 1000, Currency: "USD", PFCPrimary: "ENTERTAINMENT"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendBreakdown(context.Background(), "user", dto.AnalyticsSpendBreakdownArgs{
		GroupBy:  "pfcPrimary",
		DateFrom: helpers.Ptr("2026-01-01"),
		DateTo:   helpers.Ptr("2026-01-31"),
	})
	if err != nil {
		t.Fatalf("GetSpendBreakdown error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected only the two spend categories, got %d: %+v", len(got.Items), got.Items)
	}
	for _, item := range got.Items {
		if taxonomy.IsNonSpendCategory(item.Key) {
			t.Fatalf("non-spend category leaked into the breakdown: %q", item.Key)
		}
	}
}

func TestGetIncomeVsExpensesNoIncome(t *testing.T) {
	// No income transactions → income=0, expenses=15000, net=-15000.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{AmountMinor: 10000, Currency: "USD", PFCPrimary: "DINING"},
			{AmountMinor: 5000, Currency: "USD", PFCPrimary: "ENTERTAINMENT"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetIncomeVsExpenses(context.Background(), "user", dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetIncomeVsExpenses error: %v", err)
	}
	if got.IncomeMinor != 0 {
		t.Fatalf("expected income=0, got %v", got.IncomeMinor)
	}
	if got.ExpensesMinor != 15000 {
		t.Fatalf("expenses mismatch: got %v", got.ExpensesMinor)
	}
	if got.NetMinor != -15000 {
		t.Fatalf("net mismatch: got %v", got.NetMinor)
	}
}

func TestGetIncomeVsExpensesNoTransactions(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	got, err := svc.GetIncomeVsExpenses(context.Background(), "user", dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-01-31",
	})
	if err != nil {
		t.Fatalf("GetIncomeVsExpenses error: %v", err)
	}
	if got.IncomeMinor != 0 || got.ExpensesMinor != 0 || got.NetMinor != 0 {
		t.Fatalf("expected all zeros, got income=%v expenses=%v net=%v", got.IncomeMinor, got.ExpensesMinor, got.NetMinor)
	}
}

func TestGetIncomeVsExpensesStoreErrorPropagates(t *testing.T) {
	store := &fakeAnalyticsStore{err: errors.New("store down")}
	svc := NewAnalyticsService(store)

	_, err := svc.GetIncomeVsExpenses(context.Background(), "user", dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: "2025-01-01",
		DateTo:   "2025-01-31",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
}
