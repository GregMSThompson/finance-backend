package services

import (
	"context"
	"errors"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
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
	accountID := "bank-1"
	merchant := "amazon"
	from := "2025-01-01"
	to := "2025-01-31"
	args := dto.TransactionListArgs{
		Pending:      &pending,
		PFCPrimaries: []string{"FOOD_AND_DRINK", "ENTERTAINMENT"},
		AccountID:    &accountID,
		Merchant:     &merchant,
		DateFrom:     &from,
		DateTo:       &to,
		OrderBy:      "date",
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
	if !store.lastQuery.Desc || store.lastQuery.Limit != 6 {
		t.Fatalf("order/limit mismatch (expected store limit=args.Limit+1 for pagination peek): %+v", store.lastQuery)
	}
	if store.lastQuery.Pending == nil || *store.lastQuery.Pending != true {
		t.Fatalf("pending mismatch: %+v", store.lastQuery.Pending)
	}
	if len(store.lastQuery.PFCPrimaries) != 2 || store.lastQuery.PFCPrimaries[0] != "FOOD_AND_DRINK" || store.lastQuery.PFCPrimaries[1] != "ENTERTAINMENT" {
		t.Fatalf("pfcPrimaries mismatch: %+v", store.lastQuery.PFCPrimaries)
	}
	if store.lastQuery.AccountID == nil || *store.lastQuery.AccountID != "bank-1" {
		t.Fatalf("accountId mismatch: %+v", store.lastQuery.AccountID)
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

func TestListTransactionsSortsByAmountDesc(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "a", AmountMinor: 5, Date: "2025-01-03"},
			{TransactionID: "b", AmountMinor: 20, Date: "2025-01-01"},
			{TransactionID: "c", AmountMinor: 10, Date: "2025-01-02"},
		},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{OrderBy: "amount", Desc: true})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	// The amount path must fetch the whole window (no limit) — the sort happens
	// in memory. (The store always streams in date order.)
	if store.lastQuery.Limit != 0 {
		t.Fatalf("amount path should fetch unlimited, got %+v", store.lastQuery)
	}
	amounts := []int64{got.Transactions[0].AmountMinor, got.Transactions[1].AmountMinor, got.Transactions[2].AmountMinor}
	if amounts[0] != 20 || amounts[1] != 10 || amounts[2] != 5 {
		t.Fatalf("expected descending amounts [20 10 5], got %v", amounts)
	}
	if got.NextCursor != nil {
		t.Fatalf("amount path must not emit a cursor, got %q", *got.NextCursor)
	}
}

func TestListTransactionsSortsByAmountAscWithLimit(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "a", AmountMinor: 5},
			{TransactionID: "b", AmountMinor: 20},
			{TransactionID: "c", AmountMinor: 10},
		},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{OrderBy: "amount", Limit: 2})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("expected top 2 after sort, got %d", len(got.Transactions))
	}
	if got.Transactions[0].AmountMinor != 5 || got.Transactions[1].AmountMinor != 10 {
		t.Fatalf("expected ascending [5 10], got [%v %v]", got.Transactions[0].AmountMinor, got.Transactions[1].AmountMinor)
	}
}

func TestListTransactionsAmountTiebreak(t *testing.T) {
	store := &fakeAnalyticsStore{
		txs: []*models.Transaction{
			{TransactionID: "a", AmountMinor: 10, Date: "2025-01-01"},
			{TransactionID: "b", AmountMinor: 10, Date: "2025-01-03"},
			{TransactionID: "c", AmountMinor: 10, Date: "2025-01-02"},
		},
	}
	svc := NewTransactionsService(store)

	got, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{OrderBy: "amount", Desc: true})
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	// Equal amounts break on date (desc): b (01-03), c (01-02), a (01-01).
	ids := []string{got.Transactions[0].TransactionID, got.Transactions[1].TransactionID, got.Transactions[2].TransactionID}
	if ids[0] != "b" || ids[1] != "c" || ids[2] != "a" {
		t.Fatalf("expected tiebreak order [b c a], got %v", ids)
	}
}

func TestListTransactionsRejectsInvalidOrderBy(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewTransactionsService(store)

	_, err := svc.ListTransactions(context.Background(), "user", dto.TransactionListArgs{OrderBy: "merchant"})
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError for invalid orderBy, got %v", err)
	}
}
