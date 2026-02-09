package services

import (
	"context"
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// --- Dependencies (minimal interfaces scoped to this service) ---

// bankPSStore keeps the service decoupled from the concrete storage implementation.
type bankPSStore interface {
	Create(ctx context.Context, uid string, bank *models.Bank) error
	List(ctx context.Context, uid string) ([]*models.Bank, error)
}

// transactionPSStore is the minimal surface required for sync operations.
type transactionPSStore interface {
	UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error
	GetCursor(ctx context.Context, uid, bankID string) (string, error)
	SetCursor(ctx context.Context, uid, bankID, cursor string) error
}

// plaidClient is the Plaid SDK adapter surface used by this service.
type plaidClient interface {
	CreateLinkToken(ctx context.Context, uid string) (linkToken string, err error)
	ExchangePublicToken(ctx context.Context, publicToken string) (itemID string, accessToken string, err error)
	SyncTransactions(ctx context.Context, bankID string, accessToken string, cursor *string) (dto.PlaidSyncPage, error)
}

type plaidService struct {
	plaid    plaidClient
	banks    bankPSStore
	txs      transactionPSStore
	clockNow func() time.Time
}

func NewPlaidService(plaid plaidClient, banks bankPSStore, txs transactionPSStore) *plaidService {
	return &plaidService{
		plaid:    plaid,
		banks:    banks,
		txs:      txs,
		clockNow: time.Now,
	}
}

func (s *plaidService) CreateLinkToken(ctx context.Context, uid string) (string, error) {
	linkToken, err := s.plaid.CreateLinkToken(ctx, uid)
	if err != nil {
		return "", err
	}
	return linkToken, nil
}

func (s *plaidService) ExchangePublicToken(ctx context.Context, uid, publicToken, institutionName string) (string, error) {
	itemID, accessToken, err := s.plaid.ExchangePublicToken(ctx, publicToken)
	if err != nil {
		return "", err
	}

	bank := &models.Bank{
		BankID:           itemID,
		Institution:      institutionName,
		Status:           "active",
		PlaidPublicToken: accessToken,
		CreatedAt:        s.clockNow(),
		UpdatedAt:        s.clockNow(),
	}
	if err := s.banks.Create(ctx, uid, bank); err != nil {
		return "", err
	}

	log := logger.FromContext(ctx)
	log.Info("bank linked", "bank_id", itemID, "institution", institutionName)
	return itemID, nil
}

func (s *plaidService) SyncTransactions(ctx context.Context, uid string, bankID *string) (dto.PlaidServiceSyncResult, error) {
	result := dto.PlaidServiceSyncResult{}
	log := logger.FromContext(ctx)

	banks, err := s.banks.List(ctx, uid)
	if err != nil {
		return result, err
	}

	banksToSync := len(banks)
	if bankID != nil {
		banksToSync = 1
	}
	log.Info("transaction sync started", "bank_count", banksToSync)

	for _, b := range banks {
		if bankID != nil && *bankID != b.BankID {
			continue
		}

		token := b.PlaidPublicToken
		if token == "" {
			return result, fmt.Errorf("plaid access token missing for bank %s", b.BankID)
		}

		storedCursor, err := s.txs.GetCursor(ctx, uid, b.BankID)
		if err != nil {
			return result, err
		}
		var cursor *string
		if storedCursor != "" {
			cursor = &storedCursor
		}

		latestCursor := storedCursor
		hasMore := true
		for hasMore {
			page, err := s.plaid.SyncTransactions(ctx, b.BankID, token, cursor)
			if err != nil {
				log.Warn("bank sync failed", "bank_id", b.BankID)
				return result, err
			}

			if len(page.Transactions) > 0 {
				if err := s.txs.UpsertBatch(ctx, uid, page.Transactions); err != nil {
					return result, err
				}
				// Optionally compute inserted/updated counts inside UpsertBatch and return them instead.
				result.TransactionsInserted += len(page.Transactions)
			}

			latestCursor = page.Cursor
			cursor = &latestCursor
			hasMore = page.HasMore
		}

		if latestCursor != "" {
			if err := s.txs.SetCursor(ctx, uid, b.BankID, latestCursor); err != nil {
				return result, err
			}
		}

		result.BanksSynced++
		if bankID != nil {
			result.Cursor = helpers.Value(cursor)
			break
		}
	}

	log.Info("transaction sync completed", "banks_synced", result.BanksSynced, "transactions_inserted", result.TransactionsInserted)
	return result, nil
}
