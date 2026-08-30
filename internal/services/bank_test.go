package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type bankFakeBankStore struct {
	list      []*models.Bank
	listErr   error
	getBank   *models.Bank
	getErr    error
	deleteErr error
	deleted   []string
}

func (f *bankFakeBankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func (f *bankFakeBankStore) Get(ctx context.Context, uid, bankID string) (*models.Bank, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getBank != nil {
		return f.getBank, nil
	}
	return &models.Bank{BankID: bankID}, nil
}

func (f *bankFakeBankStore) Delete(ctx context.Context, uid, bankID string) error {
	f.deleted = append(f.deleted, uid+":"+bankID)
	return f.deleteErr
}

type bankFakePlaidRemover struct {
	calls []string
	err   error
}

func (f *bankFakePlaidRemover) RemoveItem(ctx context.Context, accessToken string) error {
	f.calls = append(f.calls, accessToken)
	return f.err
}

type bankFakeOrphanRecorder struct {
	recorded []*models.OrphanedBank
	err      error
}

func (f *bankFakeOrphanRecorder) Record(ctx context.Context, item *models.OrphanedBank) error {
	if f.err != nil {
		return f.err
	}
	f.recorded = append(f.recorded, item)
	return nil
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
	svc := NewBankService(&bankFakeBankStore{list: expected}, &bankFakeTxStore{}, &bankFakeAccountStore{}, &bankFakePlaidRemover{}, &bankFakeOrphanRecorder{}, &fakeJobs{})

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
	svc := NewBankService(&bankFakeBankStore{}, &bankFakeTxStore{}, &bankFakeAccountStore{}, &bankFakePlaidRemover{}, &bankFakeOrphanRecorder{}, jobs)

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
	banks := &bankFakeBankStore{getBank: &models.Bank{BankID: "bank-1", PlaidAccessToken: "at-123"}}
	txs := &bankFakeTxStore{}
	accounts := &bankFakeAccountStore{}
	plaid := &bankFakePlaidRemover{}
	svc := NewBankService(banks, txs, accounts, plaid, &bankFakeOrphanRecorder{}, &fakeJobs{})

	ctx := helpers.TestCtx()
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != nil {
		t.Fatalf("RunDelete returned error: %v", err)
	}
	if len(plaid.calls) != 1 || plaid.calls[0] != "at-123" {
		t.Fatalf("expected Plaid RemoveItem called with the access token, got %#v", plaid.calls)
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

func TestBankServiceRunDeleteRetriesOnTransientPlaidError(t *testing.T) {
	transient := errs.NewExternalServiceError("plaid", "boom", true, errors.New("503"))
	banks := &bankFakeBankStore{getBank: &models.Bank{BankID: "bank-1", PlaidAccessToken: "at-123"}}
	txs := &bankFakeTxStore{}
	plaid := &bankFakePlaidRemover{err: transient}
	orphans := &bankFakeOrphanRecorder{}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, plaid, orphans, &fakeJobs{})

	ctx := helpers.TestCtx()
	// A transient Plaid failure must surface so the job retries — nothing local
	// deleted, and not recorded as orphaned (a retry will likely succeed).
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != transient {
		t.Fatalf("RunDelete error = %v, want the transient error", err)
	}
	if len(txs.calls) != 0 {
		t.Fatalf("expected no local deletes on transient Plaid failure, got %#v", txs.calls)
	}
	if len(banks.deleted) != 0 {
		t.Fatalf("expected no bank delete on transient Plaid failure, got %#v", banks.deleted)
	}
	if len(orphans.recorded) != 0 {
		t.Fatalf("expected no orphan record on transient failure, got %#v", orphans.recorded)
	}
}

func TestBankServiceRunDeleteRecordsOrphanAndProceedsOnHardPlaidError(t *testing.T) {
	hard := errs.NewExternalServiceError("plaid", "invalid token", false, errors.New("400"))
	banks := &bankFakeBankStore{getBank: &models.Bank{BankID: "bank-1", Institution: "Chase", PlaidAccessToken: "at-123"}}
	txs := &bankFakeTxStore{}
	accounts := &bankFakeAccountStore{}
	plaid := &bankFakePlaidRemover{err: hard}
	orphans := &bankFakeOrphanRecorder{}
	svc := NewBankService(banks, txs, accounts, plaid, orphans, &fakeJobs{})

	ctx := helpers.TestCtx()
	// A hard Plaid failure must not block the user's delete: the Item is parked for
	// later removal (preserving the token) and the local cascade still completes.
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != nil {
		t.Fatalf("RunDelete returned error on hard Plaid failure: %v", err)
	}
	if len(orphans.recorded) != 1 {
		t.Fatalf("expected the item recorded as orphaned, got %#v", orphans.recorded)
	}
	rec := orphans.recorded[0]
	if rec.BankID != "bank-1" || rec.UID != "uid-1" || rec.Institution != "Chase" || rec.AccessToken != "at-123" || rec.Error == "" {
		t.Fatalf("orphan record missing context: %#v", rec)
	}
	if len(txs.calls) != 2 || len(accounts.deleted) != 1 || len(banks.deleted) != 1 {
		t.Fatalf("expected local cascade to complete; txs=%#v accounts=%#v banks=%#v",
			txs.calls, accounts.deleted, banks.deleted)
	}
}

func TestBankServiceRunDeleteAbortsWhenOrphanRecordingFails(t *testing.T) {
	hard := errs.NewExternalServiceError("plaid", "invalid token", false, errors.New("400"))
	recErr := errors.New("firestore write failed")
	banks := &bankFakeBankStore{getBank: &models.Bank{BankID: "bank-1", PlaidAccessToken: "at-123"}}
	txs := &bankFakeTxStore{}
	plaid := &bankFakePlaidRemover{err: hard}
	orphans := &bankFakeOrphanRecorder{err: recErr}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, plaid, orphans, &fakeJobs{})

	ctx := helpers.TestCtx()
	// If we can't even record the orphan, proceeding would discard the token, so
	// the whole delete aborts (and Cloud Tasks retries) with nothing deleted.
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != recErr {
		t.Fatalf("RunDelete error = %v, want the recording error", err)
	}
	if len(txs.calls) != 0 || len(banks.deleted) != 0 {
		t.Fatalf("expected nothing deleted locally; txs=%#v banks=%#v", txs.calls, banks.deleted)
	}
}

func TestBankServiceRunDeleteSkipsRemovalWhenBankAlreadyGone(t *testing.T) {
	banks := &bankFakeBankStore{getErr: errs.NewNotFoundError("bank not found")}
	txs := &bankFakeTxStore{}
	plaid := &bankFakePlaidRemover{}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, plaid, &bankFakeOrphanRecorder{}, &fakeJobs{})

	ctx := helpers.TestCtx()
	// A retry after the bank was already deleted locally: nothing to remove at
	// Plaid, and the idempotent local cascade still completes without error.
	if _, err := svc.RunDelete(ctx, "uid-1", dto.BankDeleteParams{BankID: "bank-1"}); err != nil {
		t.Fatalf("RunDelete returned error: %v", err)
	}
	if len(plaid.calls) != 0 {
		t.Fatalf("expected no Plaid removal for an already-gone bank, got %#v", plaid.calls)
	}
	if len(txs.calls) != 2 {
		t.Fatalf("expected the local cascade to still run, got %#v", txs.calls)
	}
}

func TestBankServiceRunDeleteStopsOnDeleteByBankError(t *testing.T) {
	expectedErr := errors.New("delete txs failed")
	banks := &bankFakeBankStore{}
	txs := &bankFakeTxStore{deleteByBankErr: expectedErr}
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, &bankFakePlaidRemover{}, &bankFakeOrphanRecorder{}, &fakeJobs{})

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
	svc := NewBankService(banks, txs, &bankFakeAccountStore{}, &bankFakePlaidRemover{}, &bankFakeOrphanRecorder{}, &fakeJobs{})

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
