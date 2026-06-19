package services

import (
	"context"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/pagination"
)

func TestListTransactionsReturnsTransactions(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "t1"},
			{TransactionID: "t2"},
		},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("transactions length mismatch: got %d", len(got.Transactions))
	}
}

func TestListTransactionsPassesFilters(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewTransactionsService(store)

	pending := true
	bankID := "bank-1"
	merchant := "amazon"
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.TransactionListArgs{
		Pending:      &pending,
		PFCPrimaries: []string{"FOOD_RETAIL", "ENTERTAINMENT"},
		BankID:       &bankID,
		Merchant:     &merchant,
		DateFrom:     &from,
		DateTo:       &to,
		OrderBy:      "amount",
		Desc:         true,
		Limit:        5,
	}

	_, err := svc.ListTransactions(context.Background(), "user-123", args)
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if store.lastUID != "user-123" {
		t.Fatalf("uid mismatch: %q", store.lastUID)
	}
	if store.lastQuery.OrderBy != "amount" || !store.lastQuery.Desc || store.lastQuery.Limit != 6 {
		t.Fatalf("order/limit mismatch (expected store limit=args.Limit+1 for pagination peek): %+v", store.lastQuery)
	}
	if store.lastQuery.Pending == nil || *store.lastQuery.Pending != true {
		t.Fatalf("pending mismatch: %+v", store.lastQuery.Pending)
	}
	if len(store.lastQuery.PFCPrimaries) != 2 || store.lastQuery.PFCPrimaries[0] != "FOOD_RETAIL" || store.lastQuery.PFCPrimaries[1] != "ENTERTAINMENT" {
		t.Fatalf("pfcPrimaries mismatch: %+v", store.lastQuery.PFCPrimaries)
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

func TestListTransactionsEmitsNextCursorWhenPageFull(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "t1"},
			{TransactionID: "t2"},
			{TransactionID: "t3"},
		},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{Limit: 2})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("expected page trimmed to 2, got %d", len(got.Transactions))
	}
	if got.NextCursor == nil {
		t.Fatalf("expected NextCursor to be set")
	}
	docID, err := pagination.DecodeCursor(*got.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if docID != "t2" {
		t.Fatalf("expected cursor to point at last returned doc t2, got %q", docID)
	}
}

func TestListTransactionsNoCursorWhenPagePartial(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{{TransactionID: "t1"}},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{Limit: 50})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if got.NextCursor != nil {
		t.Fatalf("expected no NextCursor, got %q", *got.NextCursor)
	}
}

func TestListTransactionsPassesCursor(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewTransactionsService(store)

	cursor := "abc"
	_, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{Limit: 10, Cursor: &cursor})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if store.lastQuery.Cursor == nil || *store.lastQuery.Cursor != "abc" {
		t.Fatalf("expected cursor passed through, got %+v", store.lastQuery.Cursor)
	}
}
