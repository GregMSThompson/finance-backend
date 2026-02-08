package services

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type bankFakeBankStore struct {
	list      []*models.Bank
	listErr   error
	deleteErr error
	deleted   []string
}

func (f *bankFakeBankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func (f *bankFakeBankStore) Delete(ctx context.Context, uid, bankID string) error {
	f.deleted = append(f.deleted, uid+":"+bankID)
	return f.deleteErr
}

type bankFakeTxStore struct {
	deleteByBankErr error
	deleteCursorErr error
	calls           []string
}

func (f *bankFakeTxStore) DeleteByBank(ctx context.Context, uid, bankID string) error {
	f.calls = append(f.calls, "txs:"+uid+":"+bankID)
	return f.deleteByBankErr
}

func (f *bankFakeTxStore) DeleteCursor(ctx context.Context, uid, bankID string) error {
	f.calls = append(f.calls, "cursor:"+uid+":"+bankID)
	return f.deleteCursorErr
}

func TestBankServiceListBanks(t *testing.T) {
	expected := []*models.Bank{{BankID: "b1"}, {BankID: "b2"}}
	svc := NewBankService(testLogger(), &bankFakeBankStore{list: expected}, &bankFakeTxStore{})

	ctx := logger.ToContext(context.Background(), testLogger())
	got, err := svc.ListBanks(ctx, "uid-1")
	if err != nil {
		t.Fatalf("ListBanks returned error: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("ListBanks = %#v, want %#v", got, expected)
	}
}

func TestBankServiceDeleteBankSuccess(t *testing.T) {
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{}
	svc := NewBankService(testLogger(), banks, txs)

	ctx := logger.ToContext(context.Background(), testLogger())
	if err := svc.DeleteBank(ctx, "uid-1", "bank-1"); err != nil {
		t.Fatalf("DeleteBank returned error: %v", err)
	}
	if len(txs.calls) != 2 {
		t.Fatalf("expected 2 tx calls, got %d", len(txs.calls))
	}
	if txs.calls[0] != "txs:uid-1:bank-1" || txs.calls[1] != "cursor:uid-1:bank-1" {
		t.Fatalf("unexpected tx call order: %#v", txs.calls)
	}
	if len(banks.deleted) != 1 || banks.deleted[0] != "uid-1:bank-1" {
		t.Fatalf("unexpected bank delete calls: %#v", banks.deleted)
	}
}

func TestBankServiceDeleteBankStopsOnDeleteByBankError(t *testing.T) {
	expectedErr := errors.New("delete txs failed")
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{deleteByBankErr: expectedErr}
	svc := NewBankService(testLogger(), banks, txs)

	ctx := logger.ToContext(context.Background(), testLogger())
	if err := svc.DeleteBank(ctx, "uid-1", "bank-1"); err != expectedErr {
		t.Fatalf("DeleteBank error = %v, want %v", err, expectedErr)
	}
	if len(txs.calls) != 1 || txs.calls[0] != "txs:uid-1:bank-1" {
		t.Fatalf("unexpected tx calls: %#v", txs.calls)
	}
	if len(banks.deleted) != 0 {
		t.Fatalf("expected no bank delete calls, got %#v", banks.deleted)
	}
}

func TestBankServiceDeleteBankStopsOnDeleteCursorError(t *testing.T) {
	expectedErr := errors.New("delete cursor failed")
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{deleteCursorErr: expectedErr}
	svc := NewBankService(testLogger(), banks, txs)

	ctx := logger.ToContext(context.Background(), testLogger())
	if err := svc.DeleteBank(ctx, "uid-1", "bank-1"); err != expectedErr {
		t.Fatalf("DeleteBank error = %v, want %v", err, expectedErr)
	}
	if len(txs.calls) != 2 {
		t.Fatalf("expected 2 tx calls, got %d", len(txs.calls))
	}
	if txs.calls[0] != "txs:uid-1:bank-1" || txs.calls[1] != "cursor:uid-1:bank-1" {
		t.Fatalf("unexpected tx call order: %#v", txs.calls)
	}
	if len(banks.deleted) != 0 {
		t.Fatalf("expected no bank delete calls, got %#v", banks.deleted)
	}
}

func testLogger() *slog.Logger {
	return slog.New(logger.NewTestHandler(slog.LevelInfo))
}
