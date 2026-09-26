package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type accountsPlaid interface {
	GetAccounts(ctx context.Context, accessToken string) ([]models.Account, error)
}

type accountsBankStore interface {
	Get(ctx context.Context, uid, bankID string) (*models.Bank, error)
	ListIDs(ctx context.Context, uid string) ([]string, error)
}

type accountsStore interface {
	UpsertBatch(ctx context.Context, uid, bankID string, accounts []models.Account) error
	List(ctx context.Context, uid, bankID string) ([]models.Account, error)
}

type accountsService struct {
	plaid    accountsPlaid
	banks    accountsBankStore
	accounts accountsStore
	jobs     jobSubmitter
}

func NewAccountsService(plaid accountsPlaid, banks accountsBankStore, accounts accountsStore, jobs jobSubmitter) *accountsService {
	return &accountsService{
		plaid:    plaid,
		banks:    banks,
		accounts: accounts,
		jobs:     jobs,
	}
}

// GetAccounts returns the stored accounts for the given bank.
func (s *accountsService) GetAccounts(ctx context.Context, uid, bankID string) ([]models.Account, error) {
	return s.accounts.List(ctx, uid, bankID)
}

// GetAllAccounts returns every stored account for the user across all their
// banks. Accounts are nested per bank in Firestore, so this fans out over the
// user's banks and concatenates the results. It fails on the first bank error
// rather than returning a partial list, since callers such as balance goals
// would silently understate a total against a partial set of accounts.
func (s *accountsService) GetAllAccounts(ctx context.Context, uid string) ([]models.Account, error) {
	bankIDs, err := s.banks.ListIDs(ctx, uid)
	if err != nil {
		return nil, err
	}

	var accounts []models.Account
	for _, bankID := range bankIDs {
		bankAccounts, err := s.accounts.List(ctx, uid, bankID)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, bankAccounts...)
	}

	return accounts, nil
}

// GetTotalBalance sums the current balance across the user's accounts, in minor
// units. When accountID is set the sum is scoped to that single account; a
// scope that matches no account is an error rather than a silent zero, so a
// balance goal against an unlinked account fails loudly instead of looking like
// it collapsed to nothing. Accounts with an unknown (nil) balance contribute
// zero.
func (s *accountsService) GetTotalBalance(ctx context.Context, uid string, accountID *string) (int64, error) {
	accounts, err := s.GetAllAccounts(ctx, uid)
	if err != nil {
		return 0, err
	}

	var total int64
	var matched int
	for _, a := range accounts {
		if accountID != nil && a.AccountID != *accountID {
			continue
		}
		matched++
		if a.BalanceCurrentMinor != nil {
			total += *a.BalanceCurrentMinor
		}
	}

	if accountID != nil && matched == 0 {
		return 0, errs.NewValidationError(fmt.Sprintf("account %s not found", *accountID))
	}

	return total, nil
}

// SyncAccounts submits an account.sync job for the given bank and returns the
// job ID. The actual sync runs asynchronously via RunSync on the worker.
func (s *accountsService) SyncAccounts(ctx context.Context, uid, bankID string) (string, error) {
	params, err := json.Marshal(dto.AccountSyncParams{BankID: bankID})
	if err != nil {
		return "", err
	}
	return s.jobs.Submit(ctx, uid, models.JobTypeAccountSync, params)
}

// RunSync fetches the current account list from Plaid for the given bank and
// upserts it into the store. Called from the worker task handler.
func (s *accountsService) RunSync(ctx context.Context, uid string, params dto.AccountSyncParams) (dto.AccountSyncResult, error) {
	result := dto.AccountSyncResult{BankID: params.BankID}
	log := logger.FromContext(ctx)

	bank, err := s.banks.Get(ctx, uid, params.BankID)
	if err != nil {
		return result, err
	}

	if bank.PlaidAccessToken == "" {
		log.Error("plaid access token missing for bank during account sync", "bank_id", params.BankID)
		return result, fmt.Errorf("plaid access token missing for bank %s", params.BankID)
	}

	accounts, err := s.plaid.GetAccounts(ctx, bank.PlaidAccessToken)
	if err != nil {
		return result, err
	}

	if err := s.accounts.UpsertBatch(ctx, uid, params.BankID, accounts); err != nil {
		return result, err
	}

	result.AccountsSynced = len(accounts)
	log.Info("account sync completed", "bank_id", params.BankID, "accounts_synced", result.AccountsSynced)
	return result, nil
}
