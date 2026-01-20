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
	txs []*models.Transaction
	err error
}

func (f *fakeAnalyticsStore) Query(ctx context.Context, uid string, q dto.TransactionQuery) (<-chan *models.Transaction, <-chan error) {
	out := make(chan *models.Transaction, len(f.txs))
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)
		for _, tx := range f.txs {
			select {
			case out <- tx:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
		if f.err != nil {
			errCh <- f.err
		}
	}()

	return out, errCh
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
