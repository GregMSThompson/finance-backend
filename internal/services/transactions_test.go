package services

import (
	"context"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
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
	primary := "food"
	bankID := "bank-1"
	merchant := "amazon"
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.TransactionListArgs{
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

	_, err := svc.ListTransactions(context.Background(), "user-123", args)
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
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
