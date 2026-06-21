package services

import (
	"context"
	"encoding/json"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type bankBSStore interface {
	List(ctx context.Context, uid string) ([]*models.Bank, error)
	Delete(ctx context.Context, uid, bankID string) error
}

type transactionBSStore interface {
	DeleteByBank(ctx context.Context, uid, bankID string) error
	DeleteCursor(ctx context.Context, uid, bankID string) error
}

type accountBSStore interface {
	DeleteByBank(ctx context.Context, uid, bankID string) error
}

type bankService struct {
	banks    bankBSStore
	txs      transactionBSStore
	accounts accountBSStore
	jobs     jobSubmitter
}

func NewBankService(banks bankBSStore, txs transactionBSStore, accounts accountBSStore, jobs jobSubmitter) *bankService {
	return &bankService{
		banks:    banks,
		txs:      txs,
		accounts: accounts,
		jobs:     jobs,
	}
}

func (s *bankService) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) {
	return s.banks.List(ctx, uid)
}

// DeleteBank submits a bank-delete job and returns its ID. The actual deletion
// runs asynchronously on the worker via RunDelete.
func (s *bankService) DeleteBank(ctx context.Context, uid, bankID string) (string, error) {
	params, err := json.Marshal(dto.BankDeleteParams{BankID: bankID})
	if err != nil {
		return "", err
	}
	return s.jobs.Submit(ctx, uid, models.JobTypeBankDelete, params)
}

// RunDelete cascades the bank deletion: transactions, cursor, then the bank doc.
// Each step is idempotent so retries (driven by Cloud Tasks on transient failure)
// pick up where the previous attempt left off.
func (s *bankService) RunDelete(ctx context.Context, uid string, params dto.BankDeleteParams) (dto.BankDeleteResult, error) {
	if err := s.txs.DeleteByBank(ctx, uid, params.BankID); err != nil {
		return dto.BankDeleteResult{}, err
	}
	if err := s.txs.DeleteCursor(ctx, uid, params.BankID); err != nil {
		return dto.BankDeleteResult{}, err
	}
	if err := s.accounts.DeleteByBank(ctx, uid, params.BankID); err != nil {
		return dto.BankDeleteResult{}, err
	}
	if err := s.banks.Delete(ctx, uid, params.BankID); err != nil {
		return dto.BankDeleteResult{}, err
	}

	log := logger.FromContext(ctx)
	log.Info("bank deleted", "bank_id", params.BankID)
	return dto.BankDeleteResult{BankID: params.BankID}, nil
}