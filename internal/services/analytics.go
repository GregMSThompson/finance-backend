package services

import (
	"context"
	"errors"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type transactionAnalyticsStore interface {
	Query(ctx context.Context, uid string, q dto.TransactionQuery) (<-chan *models.Transaction, <-chan error)
}

type analyticsService struct {
	txs transactionAnalyticsStore
}

func NewAnalyticsService(txs transactionAnalyticsStore) *analyticsService {
	return &analyticsService{txs: txs}
}

func (s *analyticsService) GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error) {
	result := dto.AnalyticsSpendTotalResult{
		From: helpers.Value(args.DateFrom),
		To:   helpers.Value(args.DateTo),
	}

	txCh, errCh := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
	})

	var total float64
	var currency string
	if err := streamTransactions(txCh, errCh, func(tx *models.Transaction) error {
		total += tx.Amount
		if currency == "" && tx.Currency != "" {
			currency = tx.Currency
		}
		return nil
	}); err != nil {
		return result, err
	}

	result.Total = total
	result.Currency = currency
	return result, nil
}

func (s *analyticsService) GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error) {
	result := dto.AnalyticsSpendBreakdownResult{
		GroupBy: args.GroupBy,
		From:    helpers.Value(args.DateFrom),
		To:      helpers.Value(args.DateTo),
	}
	if err := validateGroupBy(args.GroupBy); err != nil {
		return result, err
	}

	txCh, errCh := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
	})

	items := map[string]*dto.AnalyticsBreakdownItem{}
	var currency string
	if err := streamTransactions(txCh, errCh, func(tx *models.Transaction) error {
		key := breakdownKey(tx, args.GroupBy)
		if key == "" {
			return nil
		}
		item, ok := items[key]
		if !ok {
			item = &dto.AnalyticsBreakdownItem{Key: key}
			items[key] = item
		}
		item.Total += tx.Amount
		item.Count++
		if currency == "" && tx.Currency != "" {
			currency = tx.Currency
		}
		return nil
	}); err != nil {
		return result, err
	}

	result.Currency = currency
	result.Items = mapBreakdownItems(items)
	return result, nil
}

func (s *analyticsService) GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error) {
	result := dto.AnalyticsTransactionsResult{}
	txCh, errCh := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
		OrderBy:    args.OrderBy,
		Desc:       args.Desc,
		Limit:      args.Limit,
	})

	var txs []models.Transaction
	if err := streamTransactions(txCh, errCh, func(tx *models.Transaction) error {
		txs = append(txs, *tx)
		return nil
	}); err != nil {
		return result, err
	}

	result.Transactions = txs
	return result, nil
}

func breakdownKey(tx *models.Transaction, groupBy string) string {
	switch groupBy {
	case "pfcPrimary":
		return tx.PFCPrimary
	case "merchant":
		return tx.Name
	case "day":
		return tx.Date
	default:
		return ""
	}
}

func mapBreakdownItems(items map[string]*dto.AnalyticsBreakdownItem) []dto.AnalyticsBreakdownItem {
	out := make([]dto.AnalyticsBreakdownItem, 0, len(items))
	for _, item := range items {
		out = append(out, *item)
	}
	return out
}

func streamTransactions(txCh <-chan *models.Transaction, errCh <-chan error, handle func(*models.Transaction) error) error {
	for txCh != nil || errCh != nil {
		select {
		case tx, ok := <-txCh:
			if !ok {
				txCh = nil
				continue
			}
			if handle == nil {
				continue
			}
			if err := handle(tx); err != nil {
				return err
			}
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var errUnsupportedGroupBy = errors.New("unsupported groupBy")

func validateGroupBy(groupBy string) error {
	switch groupBy {
	case "pfcPrimary", "merchant", "day":
		return nil
	default:
		return errUnsupportedGroupBy
	}
}
