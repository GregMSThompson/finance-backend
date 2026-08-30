package services

import (
	"context"
	"encoding/json"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type bankBSStore interface {
	List(ctx context.Context, uid string) ([]*models.Bank, error)
	Get(ctx context.Context, uid, bankID string) (*models.Bank, error)
	Delete(ctx context.Context, uid, bankID string) error
}

// bankItemRemover severs a bank's Plaid Item so no further webhooks arrive and
// the access token is invalidated. Implemented by the Plaid adapter.
type bankItemRemover interface {
	RemoveItem(ctx context.Context, accessToken string) error
}

// orphanedBankRecorder parks a Plaid Item we couldn't remove so it can be
// removed later. Implemented by the orphaned-bank store.
type orphanedBankRecorder interface {
	Record(ctx context.Context, item *models.OrphanedBank) error
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
	plaid    bankItemRemover
	orphans  orphanedBankRecorder
	jobs     jobSubmitter
}

func NewBankService(banks bankBSStore, txs transactionBSStore, accounts accountBSStore, plaid bankItemRemover, orphans orphanedBankRecorder, jobs jobSubmitter) *bankService {
	return &bankService{
		banks:    banks,
		txs:      txs,
		accounts: accounts,
		plaid:    plaid,
		orphans:  orphans,
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

// RunDelete cascades the bank deletion: it first removes the Plaid Item (severing
// the connection and stopping webhooks), then deletes transactions, cursor,
// accounts, and the bank doc. Each step is idempotent so retries (driven by Cloud
// Tasks on transient failure) pick up where the previous attempt left off.
func (s *bankService) RunDelete(ctx context.Context, uid string, params dto.BankDeleteParams) (dto.BankDeleteResult, error) {
	log := logger.FromContext(ctx)

	// Remove the Plaid Item before touching local data. A missing bank means a
	// prior attempt already removed and deleted it, so there is nothing to do.
	bank, err := s.banks.Get(ctx, uid, params.BankID)
	switch {
	case err == nil:
		if err := s.removeOrRecordItem(ctx, uid, bank); err != nil {
			return dto.BankDeleteResult{}, err
		}
	case errs.IsNotFound(err):
		// Already deleted on a previous attempt; nothing at Plaid to remove.
	default:
		return dto.BankDeleteResult{}, err
	}

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

	log.Info("bank deleted", "bank_id", params.BankID)
	return dto.BankDeleteResult{BankID: params.BankID}, nil
}

// removeOrRecordItem removes the bank's Plaid Item. It returns an error only when
// the caller must NOT proceed with local deletion — either a transient Plaid
// failure (so Cloud Tasks retries the whole job) or a failure to record an
// unremovable Item (proceeding would discard the access token and orphan the Item
// unrecoverably). On a hard Plaid failure the Item is parked in the orphaned-bank
// collection for later removal and nil is returned, so the user's delete still
// completes rather than being blocked on Plaid.
func (s *bankService) removeOrRecordItem(ctx context.Context, uid string, bank *models.Bank) error {
	rmErr := s.plaid.RemoveItem(ctx, bank.PlaidAccessToken)
	if rmErr == nil {
		return nil
	}
	if errs.IsTransient(rmErr) {
		return rmErr
	}

	// Hard failure: we can't remove the Item now and must not block the user's
	// delete, but discarding the token would orphan the Item unrecoverably — park
	// it for later removal instead.
	orphan := &models.OrphanedBank{
		BankID:      bank.BankID,
		UID:         uid,
		Institution: bank.Institution,
		AccessToken: bank.PlaidAccessToken,
		Error:       rmErr.Error(),
	}
	if recErr := s.orphans.Record(ctx, orphan); recErr != nil {
		return recErr
	}

	logger.FromContext(ctx).Warn("plaid item removal failed; recorded for later removal",
		"bank_id", bank.BankID, "error", rmErr)
	return nil
}
