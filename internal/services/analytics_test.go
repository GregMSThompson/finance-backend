package services

import (
	"context"
	"errors"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
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
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.AnalyticsTransactionsArgs{
		Pending:    &pending,
		PFCPrimary: &primary,
		BankID:     &bankID,
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
	if store.lastQuery.DateFrom == nil || *store.lastQuery.DateFrom != "2025-01-01" {
		t.Fatalf("dateFrom mismatch: %+v", store.lastQuery.DateFrom)
	}
	if store.lastQuery.DateTo == nil || *store.lastQuery.DateTo != "2025-01-31" {
		t.Fatalf("dateTo mismatch: %+v", store.lastQuery.DateTo)
	}
}
