package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
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

type bankFakeAccountStore struct {
	deleteByBankErr error
	deleted         []string
}

func (f *bankFakeAccountStore) DeleteByBank(ctx context.Context, uid, bankID string) error {
	f.deleted = append(f.deleted, uid+":"+bankID)
	return f.deleteByBankErr
}

func TestBankServiceListBanks(t *testing.T) {
	expected := []*models.Bank{{BankID: "b1"}, {BankID: "b2"}}
	svc := NewBankService(&bankFakeBankStore{list: expected}, &bankFakeTxStore{}, &bankFakeAccountStore{}, &fakeJobs{})

	ctx := helpers.TestCtx()
	got, err := svc.ListBanks(ctx, "uid-1")
	if err != nil {
		t.Fatalf("ListBanks returned error: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("ListBanks = %#v, want %#v", got, expected)
	}
}

func TestBankServiceDeleteBankSubmitsJob(t *testing.T) {
	jobs := &fakeJobs{jobID: "job-del"}
	svc := NewBankService(&bankFakeBankStore{}, &bankFakeTxStore{}, &bankFakeAccountStore{}, jobs)

	ctx := helpers.TestCtx()
	got, err := svc.DeleteBank(ctx, "uid-1", "bank-1")
	if err != nil {
		t.Fatalf("DeleteBank returned error: %v", err)
	}
	if got != "job-del" {
		t.Fatalf("DeleteBank returned %q, want %q", got, "job-del")
	}
	if jobs.gotUID != "uid-1" || jobs.gotTyp != models.JobTypeBankDelete {
		t.Fatalf("submit called with uid=%q type=%q", jobs.gotUID, jobs.gotTyp)
	}
	var params dto.BankDeleteParams
	if err := json.Unmarshal(jobs.gotRaw, &params); err != nil {
		t.Fatalf("params not valid json: %v", err)
	}
	if params.BankID != "bank-1" {
		t.Fatalf("expected params.BankID=bank-1, got %q", params.BankID)
	}
}

func TestBankServiceRunDeleteSuccess(t *testing.T) {
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{}
	accounts := &bankFakeAccountStore{}
	svc := NewBankService(banks, txs, accounts, &fakeJobs{})

	ctx := helpers.TestCtx()
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != nil {
		t.Fatalf("RunDelete returned error: %v", err)
	}
	if len(txs.calls) != 2 {
		t.Fatalf("expected 2 tx calls, got %d", len(txs.calls))
	}
	if txs.calls[0] != "txs:uid-1:bank-1" || txs.calls[1] != "cursor:uid-1:bank-1" {
		t.Fatalf("unexpected tx call order: %#v", txs.calls)
	}
	if len(accounts.deleted) != 1 || accounts.deleted[0] != "uid-1:bank-1" {
		t.Fatalf("unexpected account delete calls: %#v", accounts.deleted)
	}
	if len(banks.deleted) != 1 || banks.deleted[0] != "uid-1:bank-1" {
		t.Fatalf("unexpected bank delete calls: %#v", banks.deleted)
	}
}

func TestBankServiceRunDeleteStopsOnDeleteByBankError(t *testing.T) {
	expectedErr := errors.New("delete txs failed")
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{deleteByBankErr: expectedErr}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, &fakeJobs{})

	ctx := helpers.TestCtx()
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != expectedErr {
		t.Fatalf("RunDelete error = %v, want %v", err, expectedErr)
	}
	if len(txs.calls) != 1 || txs.calls[0] != "txs:uid-1:bank-1" {
		t.Fatalf("unexpected tx calls: %#v", txs.calls)
	}
	if len(banks.deleted) != 0 {
		t.Fatalf("expected no bank delete calls, got %#v", banks.deleted)
	}
}

func TestBankServiceRunDeleteStopsOnDeleteCursorError(t *testing.T) {
	expectedErr := errors.New("delete cursor failed")
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{deleteCursorErr: expectedErr}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, &fakeJobs{})

	ctx := helpers.TestCtx()
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != expectedErr {
		t.Fatalf("RunDelete error = %v, want %v", err, expectedErr)
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
