package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- Dependencies (minimal interfaces scoped to this service) ---

// BankStore is the slice of methods Plaid needs; Firestore impl just has to satisfy this.
type BankStore interface {
	Create(ctx context.Context, uid string, bank *models.Bank) error
	List(ctx context.Context, uid string) ([]*models.Bank, error)
}

// PlaidSecretsStore hides Secret Manager details.
type PlaidSecretsStore interface {
	StorePlaidToken(ctx context.Context, uid, itemID, token string) error
	GetPlaidToken(ctx context.Context, uid, itemID string) (string, error)
	DeletePlaidToken(ctx context.Context, uid, itemID string) error
}

// TransactionStore is what sync uses; shape it to your Firestore model.
type TransactionStore interface {
	UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error
	GetCursor(ctx context.Context, uid, bankID string) (string, error)
	SetCursor(ctx context.Context, uid, bankID, cursor string) error
}

// PlaidClient plaid sdk adapter
type PlaidClient interface {
	CreateLinkToken(ctx context.Context, uid string) (linkToken string, err error)
	ExchangePublicToken(ctx context.Context, publicToken string) (itemID string, accessToken string, err error)
	SyncTransactions(ctx context.Context, bankID string, accessToken string, cursor *string) (dto.PlaidSyncPage, error)
}

type plaidService struct {
	log      *slog.Logger
	plaid    PlaidClient
	banks    BankStore
	secrets  PlaidSecretsStore
	txs      TransactionStore
	clockNow func() time.Time
}

func NewPlaidService(log *slog.Logger, plaid PlaidClient, banks BankStore, secrets PlaidSecretsStore, txs TransactionStore) *plaidService {
	return &plaidService{
		log:      log,
		plaid:    plaid,
		banks:    banks,
		secrets:  secrets,
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

	if err := s.secrets.StorePlaidToken(ctx, uid, itemID, accessToken); err != nil {
		return "", err
	}

	bank := &models.Bank{
		BankID:      itemID,
		Institution: institutionName,
		Status:      "active",
		CreatedAt:   s.clockNow(),
		UpdatedAt:   s.clockNow(),
	}
	if err := s.banks.Create(ctx, uid, bank); err != nil {
		return "", err
	}

	s.log.Info("Plaid item linked", "uid", uid, "itemId", itemID)
	return itemID, nil
}

func (s *plaidService) SyncTransactions(ctx context.Context, uid string, bankID *string) (dto.PlaidServiceSyncResult, error) {
	result := dto.PlaidServiceSyncResult{}

	banks, err := s.banks.List(ctx, uid)
	if err != nil {
		return result, err
	}
	for _, b := range banks {
		if bankID != nil && *bankID != b.BankID {
			continue
		}

		token, err := s.secrets.GetPlaidToken(ctx, uid, b.BankID)
		if err != nil {
			return result, err
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
			result.Cursor = deref(cursor)
			break
		}
	}

	return result, nil
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
