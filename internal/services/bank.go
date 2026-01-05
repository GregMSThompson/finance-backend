package services

import (
	"context"
	"log/slog"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type bankStore interface {
	List(ctx context.Context, uid string) ([]*models.Bank, error)
	Delete(ctx context.Context, uid, bankID string) error
}

type plaidSecrets interface {
	DeletePlaidToken(ctx context.Context, uid, itemID string) error
}

type bankService struct {
	log     *slog.Logger
	banks   bankStore
	secrets plaidSecrets
}

func NewBankService(log *slog.Logger, banks bankStore, secrets plaidSecrets) *bankService {
	return &bankService{
		log:     log,
		banks:   banks,
		secrets: secrets,
	}
}

func (s *bankService) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) {
	return s.banks.List(ctx, uid)
}

func (s *bankService) DeleteBank(ctx context.Context, uid, bankID string) error {
	if err := s.secrets.DeletePlaidToken(ctx, uid, bankID); err != nil {
		return err
	}

	if err := s.banks.Delete(ctx, uid, bankID); err != nil {
		return err
	}

	s.log.Info("Bank deleted", "uid", uid, "bankId", bankID)
	return nil
}
