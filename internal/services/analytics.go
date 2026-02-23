package services

import (
	"context"
	"sync"

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

	data, err := collectPeriod(ctx, s.txs, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
	}, args.GroupBy)
	if err != nil {
		return result, err
	}

	result.Currency = data.currency
	result.Items = mapBreakdownItems(data.items)
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

func (s *analyticsService) GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error) {
	result := dto.AnalyticsPeriodComparisonResult{
		GroupBy: args.GroupBy,
	}

	if args.GroupBy != "" {
		if err := validateGroupBy(args.GroupBy); err != nil {
			return result, err
		}
	}

	currentQuery := dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   &args.CurrentFrom,
		DateTo:     &args.CurrentTo,
	}
	previousQuery := dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   &args.PreviousFrom,
		DateTo:     &args.PreviousTo,
	}

	var wg sync.WaitGroup
	var currentData, previousData periodData
	var currentErr, previousErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		currentData, currentErr = collectPeriod(ctx, s.txs, uid, currentQuery, args.GroupBy)
	}()
	go func() {
		defer wg.Done()
		previousData, previousErr = collectPeriod(ctx, s.txs, uid, previousQuery, args.GroupBy)
	}()
	wg.Wait()

	if currentErr != nil {
		return result, currentErr
	}
	if previousErr != nil {
		return result, previousErr
	}

	var currentItems, previousItems []dto.AnalyticsBreakdownItem
	if args.GroupBy != "" {
		currentItems = mapBreakdownItems(currentData.items)
		previousItems = mapBreakdownItems(previousData.items)
	}

	result.Current = dto.PeriodSummary{
		Total:    currentData.total,
		Count:    currentData.count,
		Currency: currentData.currency,
		From:     args.CurrentFrom,
		To:       args.CurrentTo,
		Items:    currentItems,
	}
	result.Previous = dto.PeriodSummary{
		Total:    previousData.total,
		Count:    previousData.count,
		Currency: previousData.currency,
		From:     args.PreviousFrom,
		To:       args.PreviousTo,
		Items:    previousItems,
	}
	result.Change = buildChange(currentData, previousData, args.GroupBy)

	return result, nil
}

// periodData holds the accumulated totals for a single query period.
type periodData struct {
	total    float64
	count    int
	currency string
	items    map[string]*dto.AnalyticsBreakdownItem
}

// collectPeriod runs a single store query and accumulates totals and an optional
// group breakdown into a periodData value.
func collectPeriod(ctx context.Context, store transactionAnalyticsStore, uid string, q dto.TransactionQuery, groupBy string) (periodData, error) {
	data := periodData{
		items: map[string]*dto.AnalyticsBreakdownItem{},
	}
	err := store.Query(ctx, uid, q, func(tx *models.Transaction) error {
		data.total += tx.Amount
		data.count++
		if data.currency == "" && tx.Currency != "" {
			data.currency = tx.Currency
		}
		if groupBy != "" {
			key := breakdownKey(tx, groupBy)
			if key != "" {
				item, ok := data.items[key]
				if !ok {
					item = &dto.AnalyticsBreakdownItem{Key: key}
					data.items[key] = item
				}
				item.Total += tx.Amount
				item.Count++
			}
		}
		return nil
	})
	return data, err
}

func buildChange(current, previous periodData, groupBy string) dto.PeriodChange {
	change := dto.PeriodChange{
		AbsoluteChange:   current.total - previous.total,
		PercentageChange: percentageChange(current.total, previous.total),
		CountChange:      current.count - previous.count,
	}

	if groupBy != "" {
		keys := map[string]bool{}
		for k := range current.items {
			keys[k] = true
		}
		for k := range previous.items {
			keys[k] = true
		}

		for key := range keys {
			var currTotal float64
			var currCount int
			var prevTotal float64
			var prevCount int

			if item := current.items[key]; item != nil {
				currTotal = item.Total
				currCount = item.Count
			}
			if item := previous.items[key]; item != nil {
				prevTotal = item.Total
				prevCount = item.Count
			}

			change.Items = append(change.Items, dto.BreakdownItemChange{
				Key:              key,
				AbsoluteChange:   currTotal - prevTotal,
				PercentageChange: percentageChange(currTotal, prevTotal),
				CountChange:      currCount - prevCount,
			})
		}
	}

	return change
}

func percentageChange(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	pct := (current - previous) / previous * 100
	return &pct
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
