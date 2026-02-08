package services

import (
	"context"

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

type bankService struct {
	banks   bankBSStore
	txs     transactionBSStore
}

func NewBankService(banks bankBSStore, txs transactionBSStore) *bankService {
	return &bankService{
		banks:   banks,
		txs:     txs,
	}
}

func (s *bankService) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) {
	return s.banks.List(ctx, uid)
}

func (s *bankService) DeleteBank(ctx context.Context, uid, bankID string) error {
	// TODO: Make deletions atomic or add retries to avoid partial cleanup on failure.
	if err := s.txs.DeleteByBank(ctx, uid, bankID); err != nil {
		return err
	}
	if err := s.txs.DeleteCursor(ctx, uid, bankID); err != nil {
		return err
	}
	if err := s.banks.Delete(ctx, uid, bankID); err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	log.Info("bank deleted", "bank_id", bankID)
	return nil
}
