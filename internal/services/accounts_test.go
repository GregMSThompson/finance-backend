package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type acctFakeBankStore struct {
	banks   []*models.Bank
	listErr error
}

func (f *acctFakeBankStore) Get(ctx context.Context, uid, bankID string) (*models.Bank, error) {
	return &models.Bank{BankID: bankID}, nil
}

func (f *acctFakeBankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	return f.banks, f.listErr
}

type acctFakeAccountStore struct {
	byBank  map[string][]models.Account
	listErr map[string]error
}

func (f *acctFakeAccountStore) UpsertBatch(ctx context.Context, uid, bankID string, accounts []models.Account) error {
	return nil
}

func (f *acctFakeAccountStore) List(ctx context.Context, uid, bankID string) ([]models.Account, error) {
	if err := f.listErr[bankID]; err != nil {
		return nil, err
	}
	return f.byBank[bankID], nil
}

func TestGetAllAccounts_FansOutOverBanks(t *testing.T) {
	banks := &acctFakeBankStore{banks: []*models.Bank{{BankID: "b1"}, {BankID: "b2"}}}
	accounts := &acctFakeAccountStore{byBank: map[string][]models.Account{
		"b1": {{AccountID: "a1"}, {AccountID: "a2"}},
		"b2": {{AccountID: "a3"}},
	}}
	svc := NewAccountsService(nil, banks, accounts, nil)

	got, err := svc.GetAllAccounts(context.Background(), "uid1")
	if err != nil {
		t.Fatalf("GetAllAccounts: %v", err)
	}

	want := []string{"a1", "a2", "a3"}
	var ids []string
	for _, a := range got {
		ids = append(ids, a.AccountID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("account ids = %v, want %v", ids, want)
	}
}

func TestGetAllAccounts_BankListError(t *testing.T) {
	sentinel := errors.New("boom")
	banks := &acctFakeBankStore{listErr: sentinel}
	svc := NewAccountsService(nil, banks, &acctFakeAccountStore{}, nil)

	if _, err := svc.GetAllAccounts(context.Background(), "uid1"); !errors.Is(err, sentinel) {
		t.Fatalf("expected bank list error, got %v", err)
	}
}

// A single bank's account query failing must fail the whole call rather than
// return a partial list, so a balance total is never computed against a subset.
func TestGetAllAccounts_FailsHardOnAccountError(t *testing.T) {
	sentinel := errors.New("boom")
	banks := &acctFakeBankStore{banks: []*models.Bank{{BankID: "b1"}, {BankID: "b2"}}}
	accounts := &acctFakeAccountStore{
		byBank:  map[string][]models.Account{"b1": {{AccountID: "a1"}}},
		listErr: map[string]error{"b2": sentinel},
	}
	svc := NewAccountsService(nil, banks, accounts, nil)

	if _, err := svc.GetAllAccounts(context.Background(), "uid1"); !errors.Is(err, sentinel) {
		t.Fatalf("expected account list error, got %v", err)
	}
}
