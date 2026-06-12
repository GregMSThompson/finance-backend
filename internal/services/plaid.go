package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// --- Dependencies (minimal interfaces scoped to this service) ---

// bankPSStore keeps the service decoupled from the concrete storage implementation.
type bankPSStore interface {
	Create(ctx context.Context, uid string, bank *models.Bank) error
	List(ctx context.Context, uid string) ([]*models.Bank, error)
	FindUserIDByBankID(ctx context.Context, bankID string) (string, error)
	SetNeedsReauth(ctx context.Context, uid, bankID string, needs bool) error
}

// bankDeleter is the bank service surface used by the webhook handler when
// Plaid signals USER_PERMISSION_REVOKED. The actual deletion runs as a job.
type bankDeleter interface {
	DeleteBank(ctx context.Context, uid, bankID string) (string, error)
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

// jobSubmitter enqueues async work via the job service.
type jobSubmitter interface {
	Submit(ctx context.Context, uid string, jobType models.JobType, params json.RawMessage) (string, error)
}

type plaidService struct {
	plaid    plaidClient
	banks    bankPSStore
	txs      transactionPSStore
	jobs     jobSubmitter
	bankSvc  bankDeleter
	clockNow func() time.Time
}

func NewPlaidService(plaid plaidClient, banks bankPSStore, txs transactionPSStore, jobs jobSubmitter, bankSvc bankDeleter) *plaidService {
	return &plaidService{
		plaid:    plaid,
		banks:    banks,
		txs:      txs,
		jobs:     jobs,
		bankSvc:  bankSvc,
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

func (s *plaidService) ExchangePublicToken(ctx context.Context, uid string, req dto.LinkBankRequest) (string, error) {
	itemID, accessToken, err := s.plaid.ExchangePublicToken(ctx, req.PublicToken)
	if err != nil {
		return "", err
	}

	bank := &models.Bank{
		BankID:           itemID,
		Institution:      req.InstitutionName,
		Status:           "active",
		PlaidPublicToken: accessToken,
		CreatedAt:        s.clockNow(),
		UpdatedAt:        s.clockNow(),
	}
	if err := s.banks.Create(ctx, uid, bank); err != nil {
		return "", err
	}

	log := logger.FromContext(ctx)
	log.Info("bank linked", "bank_id", itemID, "institution", req.InstitutionName)
	return itemID, nil
}

// HandleWebhook is called by the webhook handler after signature verification.
// It resolves the item to its owning uid and dispatches based on the webhook code.
// Most branches end up submitting a job or updating bank state; the handler
// always returns 200 to Plaid unless this method returns an error.
func (s *plaidService) HandleWebhook(ctx context.Context, p dto.PlaidWebhook) error {
	log := logger.FromContext(ctx)

	uid, err := s.banks.FindUserIDByBankID(ctx, p.ItemID)
	if err != nil {
		if errs.IsNotFound(err) {
			// Plaid sometimes delivers events for items we've already removed locally.
			// Ack and move on so Plaid stops retrying.
			log.Warn("webhook for unknown item", "item_id", p.ItemID)
			return nil
		}
		return err
	}

	switch fmt.Sprintf("%s:%s", p.WebhookType, p.WebhookCode) {
	case "TRANSACTIONS:SYNC_UPDATES_AVAILABLE":
		bankID := p.ItemID
		_, err := s.SyncTransactions(ctx, uid, dto.SyncTransactionsRequest{BankID: &bankID})
		return err

	case "ITEM:ERROR":
		if p.Error != nil && p.Error.ErrorCode == "ITEM_LOGIN_REQUIRED" {
			return s.banks.SetNeedsReauth(ctx, uid, p.ItemID, true)
		}
		return nil

	case "ITEM:PENDING_EXPIRATION", "ITEM:PENDING_DISCONNECT":
		return s.banks.SetNeedsReauth(ctx, uid, p.ItemID, true)

	case "ITEM:LOGIN_REPAIRED":
		return s.banks.SetNeedsReauth(ctx, uid, p.ItemID, false)

	case "ITEM:USER_PERMISSION_REVOKED":
		_, err := s.bankSvc.DeleteBank(ctx, uid, p.ItemID)
		return err

	default:
		log.Info("unhandled plaid webhook", "type", p.WebhookType, "code", p.WebhookCode)
		return nil
	}
}

// SyncTransactions submits a sync job and returns its ID. The actual sync runs
// asynchronously on the worker via RunSync.
func (s *plaidService) SyncTransactions(ctx context.Context, uid string, req dto.SyncTransactionsRequest) (string, error) {
	params, err := json.Marshal(dto.PlaidSyncParams{BankID: req.BankID})
	if err != nil {
		return "", err
	}
	return s.jobs.Submit(ctx, uid, models.JobTypePlaidSync, params)
}

// RunSync performs the transaction sync. Called from the worker task handler.
// If params.BankID is set, only that bank is synced; otherwise all banks for the user.
func (s *plaidService) RunSync(ctx context.Context, uid string, params dto.PlaidSyncParams) (dto.PlaidServiceSyncResult, error) {
	result := dto.PlaidServiceSyncResult{}
	log := logger.FromContext(ctx)

	banks, err := s.banks.List(ctx, uid)
	if err != nil {
		return result, err
	}

	banksToSync := len(banks)
	if params.BankID != nil {
		banksToSync = 1
	}
	log.Info("transaction sync started", "bank_count", banksToSync)

	for _, b := range banks {
		if params.BankID != nil && *params.BankID != b.BankID {
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
		if params.BankID != nil {
			result.Cursor = helpers.Value(cursor)
			break
		}
	}

	log.Info("transaction sync completed", "banks_synced", result.BanksSynced, "transactions_inserted", result.TransactionsInserted)
	return result, nil
}
