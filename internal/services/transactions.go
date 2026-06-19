package services

import (
	"context"
	"fmt"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/pagination"
)

// pfcPrimariesLimit caps the number of category filters per call so we stay
// within Firestore's `in` operator limit (currently 30 values).
const pfcPrimariesLimit = 30

type transactionStore interface {
	Query(ctx context.Context, uid string, q dto.TransactionQuery, handle func(*models.Transaction) error) error
}

type transactionsService struct {
	txs transactionStore
}

func NewTransactionsService(txs transactionStore) *transactionsService {
	return &transactionsService{txs: txs}
}

func (s *transactionsService) ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error) {
	result := dto.TransactionListResult{}

	if err := validatePFCPrimaries(args.PFCPrimaries); err != nil {
		return result, err
	}

	// Fetch one extra row to detect whether more pages exist without an
	// additional round-trip. The extra row is trimmed before returning;
	// its presence tells us to emit a NextCursor.
	storeLimit := args.Limit
	if storeLimit > 0 {
		storeLimit++
	}

	var txs []models.Transaction
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:      args.Pending,
		PFCPrimaries: args.PFCPrimaries,
		BankID:       args.BankID,
		Merchant:     args.Merchant,
		DateFrom:     args.DateFrom,
		DateTo:       args.DateTo,
		OrderBy:      args.OrderBy,
		Desc:         args.Desc,
		Limit:        storeLimit,
		Cursor:       args.Cursor,
	}, func(tx *models.Transaction) error {
		txs = append(txs, *tx)
		return nil
	}); err != nil {
		return result, err
	}

	if args.Limit > 0 && len(txs) > args.Limit {
		txs = txs[:args.Limit]
		cursor := pagination.EncodeCursor(txs[len(txs)-1].TransactionID)
		result.NextCursor = &cursor
	}

	result.Transactions = txs
	return result, nil
}

func validatePFCPrimaries(primaries []string) error {
	if len(primaries) > pfcPrimariesLimit {
		return errs.NewValidationError(fmt.Sprintf("too many category filters: %d (max %d)", len(primaries), pfcPrimariesLimit))
	}
	for _, p := range primaries {
		if !taxonomy.IsPFCPrimaryAllowed(p) {
			return errs.NewValidationError(fmt.Sprintf("invalid category: %s", p))
		}
	}
	return nil
}
