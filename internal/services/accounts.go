package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type accountsPlaid interface {
	GetAccounts(ctx context.Context, accessToken string) ([]models.Account, error)
}

type accountsBankStore interface {
	Get(ctx context.Context, uid, bankID string) (*models.Bank, error)
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

	if bank.PlaidPublicToken == "" {
		log.Error("plaid access token missing for bank during account sync", "bank_id", params.BankID)
		return result, fmt.Errorf("plaid access token missing for bank %s", params.BankID)
	}

	accounts, err := s.plaid.GetAccounts(ctx, bank.PlaidPublicToken)
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
