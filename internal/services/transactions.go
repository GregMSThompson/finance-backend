package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/pagination"
)

// pfcPrimariesLimit caps the number of category filters per call so we stay
// within Firestore's `in` operator limit (currently 30 values).
const pfcPrimariesLimit = 30

// sortableTransactionFields lists the fields transactions may be ordered by.
// `date` is served natively by Firestore; `amount` is sorted in memory (see
// ListTransactions) because a range filter on `date` precludes ordering by a
// second field without a combinatorial explosion of composite indexes.
func sortableTransactionFields() []string {
	return []string{"date", "amount"}
}

func isSortableTransactionField(field string) bool {
	for _, f := range sortableTransactionFields() {
		if field == f {
			return true
		}
	}
	return false
}

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
	if args.OrderBy != "" && !isSortableTransactionField(args.OrderBy) {
		return result, errs.NewValidationError(fmt.Sprintf("invalid orderBy field: %s", args.OrderBy))
	}

	// The store always streams in date order. Amount sorting is therefore done
	// in memory below: fetch the whole window (no limit, no cursor) and re-sort.
	// Callers bound the read via the date range (see the AI service's max-range
	// guard).
	byAmount := args.OrderBy == "amount"

	storeLimit := args.Limit
	storeCursor := args.Cursor
	if byAmount {
		storeLimit = 0
		storeCursor = nil
	} else if storeLimit > 0 {
		// Fetch one extra row to detect a next page without a second round-trip.
		storeLimit++
	}

	txs := make([]models.Transaction, 0)
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:      args.Pending,
		PFCPrimaries: args.PFCPrimaries,
		AccountID:    args.AccountID,
		Merchant:     args.Merchant,
		DateFrom:     args.DateFrom,
		DateTo:       args.DateTo,
		Desc:         args.Desc,
		Limit:        storeLimit,
		Cursor:       storeCursor,
	}, func(tx *models.Transaction) error {
		txs = append(txs, *tx)
		return nil
	}); err != nil {
		return result, err
	}

	if byAmount {
		sortTransactionsByAmount(txs, args.Desc)
		if args.Limit > 0 && len(txs) > args.Limit {
			txs = txs[:args.Limit]
		}
		result.Transactions = txs
		return result, nil
	}

	if args.Limit > 0 && len(txs) > args.Limit {
		txs = txs[:args.Limit]
		cursor := pagination.EncodeCursor(txs[len(txs)-1].TransactionID)
		result.NextCursor = &cursor
	}

	result.Transactions = txs
	return result, nil
}

// sortTransactionsByAmount sorts in place by amount (descending when desc is
// true), tie-breaking on date (most recent first) then transaction id for a
// stable, repeatable order.
func sortTransactionsByAmount(txs []models.Transaction, desc bool) {
	sort.Slice(txs, func(i, j int) bool {
		a, b := txs[i], txs[j]
		if a.Amount != b.Amount {
			if desc {
				return a.Amount > b.Amount
			}
			return a.Amount < b.Amount
		}
		if a.Date != b.Date {
			return a.Date > b.Date
		}
		return a.TransactionID < b.TransactionID
	})
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
