package services

import (
	"context"
	"log/slog"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type bankBSStore interface {
	List(ctx context.Context, uid string) ([]*models.Bank, error)
	Delete(ctx context.Context, uid, bankID string) error
}

type bankService struct {
	log     *slog.Logger
	banks   bankBSStore
}

func NewBankService(log *slog.Logger, banks bankBSStore) *bankService {
	return &bankService{
		log:     log,
		banks:   banks,
	}
}

func (s *bankService) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) {
	return s.banks.List(ctx, uid)
}

func (s *bankService) DeleteBank(ctx context.Context, uid, bankID string) error {
	if err := s.banks.Delete(ctx, uid, bankID); err != nil {
		return err
	}

	s.log.Info("Bank deleted", "uid", uid, "bankId", bankID)
	return nil
}
