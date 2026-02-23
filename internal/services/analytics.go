package services

import (
	"context"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type transactionAnalyticsStore interface {
	Query(ctx context.Context, uid string, q dto.TransactionQuery, handle func(*models.Transaction) error) error
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

	var total float64
	var currency string
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
	}, func(tx *models.Transaction) error {
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

	items := map[string]*dto.AnalyticsBreakdownItem{}
	var currency string
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
	}, func(tx *models.Transaction) error {
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

	var txs []models.Transaction
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
		OrderBy:    args.OrderBy,
		Desc:       args.Desc,
		Limit:      args.Limit,
	}, func(tx *models.Transaction) error {
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

func validateGroupBy(groupBy string) error {
	switch groupBy {
	case "pfcPrimary", "merchant", "day":
		return nil
	default:
		return errs.NewUnsupportedGroupByError()
	}
}
