package services

import (
	"context"
	"errors"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type fakeAnalyticsStore struct {
	txs      []*models.Transaction
	err      error
	lastUID  string
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
			{Amount: 10.5, Currency: "USD"},
			{Amount: 2.25, Currency: "USD"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetSpendTotal(context.Background(), "user", dto.AnalyticsSpendTotalArgs{})
	if err != nil {
		t.Fatalf("GetSpendTotal error: %v", err)
	}

	if got.Total != 12.75 {
		t.Fatalf("total mismatch: got %v", got.Total)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: got %q", got.Currency)
	}
}

func TestAnalyticsSpendBreakdown(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Coffee", Amount: 3, Currency: "USD"},
			{Name: "Coffee", Amount: 2, Currency: "USD"},
			{Name: "Lunch", Amount: 8, Currency: "USD"},
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
	if items["Coffee"].Total != 5 || items["Coffee"].Count != 2 {
		t.Fatalf("coffee totals mismatch: %+v", items["Coffee"])
	}
	if items["Lunch"].Total != 8 || items["Lunch"].Count != 1 {
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

func TestAnalyticsTransactions(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "t1"},
			{TransactionID: "t2"},
		},
	}
	svc := NewAnalyticsService(store)

	got, err := svc.GetTransactions(context.Background(), "user", dto.AnalyticsTransactionsArgs{})
	if err != nil {
		t.Fatalf("GetTransactions error: %v", err)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("transactions length mismatch: got %d", len(got.Transactions))
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

func TestAnalyticsTransactionsPassesFilters(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(store)

	pending := true
	primary := "food"
	bankID := "bank-1"
	merchant := "amazon"
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.AnalyticsTransactionsArgs{
		Pending:    &pending,
		PFCPrimary: &primary,
		BankID:     &bankID,
		Merchant:   &merchant,
		DateFrom:   &from,
		DateTo:     &to,
		OrderBy:    "amount",
		Desc:       true,
		Limit:      5,
	}

	_, err := svc.GetTransactions(context.Background(), "user-123", args)
	if err != nil {
		t.Fatalf("GetTransactions error: %v", err)
	}
	if store.lastUID != "user-123" {
		t.Fatalf("uid mismatch: %q", store.lastUID)
	}
	if store.lastQuery.OrderBy != "amount" || !store.lastQuery.Desc || store.lastQuery.Limit != 5 {
		t.Fatalf("order/limit mismatch: %+v", store.lastQuery)
	}
	if store.lastQuery.Pending == nil || *store.lastQuery.Pending != true {
		t.Fatalf("pending mismatch: %+v", store.lastQuery.Pending)
	}
	if store.lastQuery.PFCPrimary == nil || *store.lastQuery.PFCPrimary != "food" {
		t.Fatalf("pfcPrimary mismatch: %+v", store.lastQuery.PFCPrimary)
	}
	if store.lastQuery.BankID == nil || *store.lastQuery.BankID != "bank-1" {
		t.Fatalf("bankId mismatch: %+v", store.lastQuery.BankID)
	}
	if store.lastQuery.Merchant == nil || *store.lastQuery.Merchant != "amazon" {
		t.Fatalf("merchant mismatch: %+v", store.lastQuery.Merchant)
	}
	if store.lastQuery.DateFrom == nil || *store.lastQuery.DateFrom != "2025-01-01" {
		t.Fatalf("dateFrom mismatch: %+v", store.lastQuery.DateFrom)
	}
	if store.lastQuery.DateTo == nil || *store.lastQuery.DateTo != "2025-01-31" {
		t.Fatalf("dateTo mismatch: %+v", store.lastQuery.DateTo)
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
					{Amount: 30, Currency: "USD"},
					{Amount: 20, Currency: "USD"},
				}, nil
			}
			return []*models.Transaction{
				{Amount: 40, Currency: "USD"},
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

	if got.Current.Total != 50 || got.Current.Count != 2 {
		t.Fatalf("current mismatch: total=%v count=%v", got.Current.Total, got.Current.Count)
	}
	if got.Previous.Total != 40 || got.Previous.Count != 1 {
		t.Fatalf("previous mismatch: total=%v count=%v", got.Previous.Total, got.Previous.Count)
	}
	if got.Change.AbsoluteChange != 10 {
		t.Fatalf("absolute change mismatch: %v", got.Change.AbsoluteChange)
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
	if got.Current.Items != nil || got.Previous.Items != nil || got.Change.Items != nil {
		t.Fatal("expected nil items when no groupBy")
	}
}

func TestGetPeriodComparisonNilPercentageWhenPreviousZero(t *testing.T) {
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return []*models.Transaction{
					{Amount: 30, Currency: "USD"},
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
	if got.Change.AbsoluteChange != 30 {
		t.Fatalf("absolute change mismatch: %v", got.Change.AbsoluteChange)
	}
}

func TestGetPeriodComparisonWithGroupBy(t *testing.T) {
	store := &funcAnalyticsStore{
		fn: func(q dto.TransactionQuery) ([]*models.Transaction, error) {
			if helpers.Value(q.DateFrom) == "2025-02-01" {
				return []*models.Transaction{
					{Name: "Coffee", Amount: 5, Currency: "USD"},
					{Name: "Lunch", Amount: 10, Currency: "USD"},
				}, nil
			}
			return []*models.Transaction{
				{Name: "Coffee", Amount: 4, Currency: "USD"},
				{Name: "Dinner", Amount: 8, Currency: "USD"},
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
	if coffee.AbsoluteChange != 1 {
		t.Fatalf("Coffee absolute change mismatch: %v", coffee.AbsoluteChange)
	}
	if coffee.PercentageChange == nil || helpers.Value(coffee.PercentageChange) != 25 {
		t.Fatalf("Coffee percentage change mismatch: %v", coffee.PercentageChange)
	}

	lunch := changeByKey["Lunch"]
	if lunch.AbsoluteChange != 10 {
		t.Fatalf("Lunch absolute change mismatch: %v", lunch.AbsoluteChange)
	}
	if lunch.PercentageChange != nil {
		t.Fatalf("Lunch expected nil percentage (previous=0), got %v", *lunch.PercentageChange)
	}

	dinner := changeByKey["Dinner"]
	if dinner.AbsoluteChange != -8 {
		t.Fatalf("Dinner absolute change mismatch: %v", dinner.AbsoluteChange)
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
			{Name: "Netflix", Amount: 15.99, Currency: "USD", Date: "2025-01-15"},
			{Name: "Netflix", Amount: 15.99, Currency: "USD", Date: "2025-02-14"},
			{Name: "Netflix", Amount: 15.99, Currency: "USD", Date: "2025-03-15"},
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
	if item.TypicalAmount != 15.99 {
		t.Fatalf("typical amount mismatch: %v", item.TypicalAmount)
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
	if item.MonthlyEquivalent != 15.99 {
		t.Fatalf("monthly equivalent mismatch: %v", item.MonthlyEquivalent)
	}
	if got.TotalMonthlyEquivalent != 15.99 {
		t.Fatalf("total monthly equivalent mismatch: %v", got.TotalMonthlyEquivalent)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency mismatch: %q", got.Currency)
	}
}

func TestGetRecurringTransactionsWeekly(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Gym", Amount: 10, Currency: "USD", Date: "2025-01-06"},
			{Name: "Gym", Amount: 10, Currency: "USD", Date: "2025-01-13"},
			{Name: "Gym", Amount: 10, Currency: "USD", Date: "2025-01-20"},
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
	// 10 * 4.33 = 43.3
	if got.TotalMonthlyEquivalent != 43.3 {
		t.Fatalf("total monthly equivalent mismatch: %v", got.TotalMonthlyEquivalent)
	}
}

func TestGetRecurringTransactionsDropsInsufficientOccurrences(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "One-off", Amount: 50, Currency: "USD", Date: "2025-01-10"},
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
			{Name: "Irregular", Amount: 20, Currency: "USD", Date: "2025-01-01"},
			{Name: "Irregular", Amount: 20, Currency: "USD", Date: "2025-03-02"}, // 60 day gap
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
			{Name: "Utility", Amount: 80, Currency: "USD", Date: "2025-01-15"},
			{Name: "Utility", Amount: 110, Currency: "USD", Date: "2025-02-15"},
			{Name: "Utility", Amount: 95, Currency: "USD", Date: "2025-03-15"},
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
			{Amount: 100, Currency: "USD", Date: "2025-01-15"},
			{Amount: 80, Currency: "USD", Date: "2025-02-15"},
			{Amount: 120, Currency: "USD", Date: "2025-03-15"},
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
	// total=300, units=90/30=3 → average=100
	if got.AveragePerUnit != 100 {
		t.Fatalf("averagePerUnit mismatch: got %v", got.AveragePerUnit)
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
	if got.Series[0].Total != 100 || got.Series[1].Total != 80 || got.Series[2].Total != 120 {
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
			{Amount: 10, Currency: "USD", Date: "2025-01-06"},
			{Amount: 20, Currency: "USD", Date: "2025-01-13"},
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
	// total=30, units=14/7=2 → average=15
	if got.AveragePerUnit != 15 {
		t.Fatalf("averagePerUnit mismatch: got %v", got.AveragePerUnit)
	}
}

func TestGetMovingAverageScopeCategory(t *testing.T) {
	// 2 categories across a 31-day window.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Amount: 50, Currency: "USD", Date: "2025-01-10", PFCPrimary: "Food"},
			{Amount: 30, Currency: "USD", Date: "2025-01-15", PFCPrimary: "Food"},
			{Amount: 20, Currency: "USD", Date: "2025-01-20", PFCPrimary: "Transport"},
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
	if got.AveragePerUnit != 0 {
		t.Fatalf("expected averagePerUnit=0, got %v", got.AveragePerUnit)
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
	// Alpha=60, Beta=30, Gamma=10, total=100. Top 2: Alpha then Beta.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Alpha", Amount: 60, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", Amount: 30, Currency: "USD", Date: "2025-01-11"},
			{Name: "Gamma", Amount: 10, Currency: "USD", Date: "2025-01-12"},
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
	if got.TotalSpend != 100 {
		t.Fatalf("totalSpend mismatch: got %v", got.TotalSpend)
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
	if got.Items[0].Total != 60 || got.Items[1].Total != 30 {
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
			{PFCPrimary: "Food", Amount: 80, Currency: "USD", Date: "2025-01-10"},
			{PFCPrimary: "Food", Amount: 20, Currency: "USD", Date: "2025-01-15"},
			{PFCPrimary: "Transport", Amount: 50, Currency: "USD", Date: "2025-01-12"},
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
	// Food=100, Transport=50 → top order.
	if got.Items[0].Key != "Food" || got.Items[1].Key != "Transport" {
		t.Fatalf("item order mismatch: %+v", got.Items)
	}
	if got.Items[0].Count != 2 || got.Items[1].Count != 1 {
		t.Fatalf("item counts mismatch: %+v", got.Items)
	}
}

func TestGetTopNBottom(t *testing.T) {
	// Alpha=60, Beta=30, Gamma=10, total=100. Bottom 2: Gamma then Beta.
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{Name: "Alpha", Amount: 60, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", Amount: 30, Currency: "USD", Date: "2025-01-11"},
			{Name: "Gamma", Amount: 10, Currency: "USD", Date: "2025-01-12"},
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
			{Name: "Alpha", Amount: 60, Currency: "USD", Date: "2025-01-10"},
			{Name: "Beta", Amount: 30, Currency: "USD", Date: "2025-01-11"},
			{Name: "Beta", Amount: 20, Currency: "USD", Date: "2025-01-15"},
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
	if got.TotalSpend != 0 {
		t.Fatalf("expected totalSpend=0, got %v", got.TotalSpend)
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
