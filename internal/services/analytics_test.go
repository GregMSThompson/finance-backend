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
