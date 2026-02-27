package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

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

func (s *analyticsService) GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error) {
	result := dto.RecurringTransactionsResult{
		Items: []dto.RecurringItem{},
		From:  args.DateFrom,
		To:    args.DateTo,
	}

	type merchantGroup struct {
		dates    []string
		amounts  []float64
		currency string
	}

	pending := false
	groups := map[string]*merchantGroup{}

	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:  &pending,
		BankID:   args.BankID,
		DateFrom: &args.DateFrom,
		DateTo:   &args.DateTo,
	}, func(tx *models.Transaction) error {
		g, ok := groups[tx.Name]
		if !ok {
			g = &merchantGroup{}
			groups[tx.Name] = g
		}
		g.dates = append(g.dates, tx.Date)
		g.amounts = append(g.amounts, tx.Amount)
		if g.currency == "" && tx.Currency != "" {
			g.currency = tx.Currency
		}
		return nil
	}); err != nil {
		return result, err
	}

	var totalMonthly float64
	var currency string

	for name, g := range groups {
		if len(g.dates) < 2 {
			continue
		}

		sort.Strings(g.dates)
		gaps, err := computeGaps(g.dates)
		if err != nil {
			return result, err
		}
		if len(gaps) == 0 {
			continue
		}

		freq := classifyFrequency(medianInt(gaps))
		if freq == "" {
			continue
		}

		typical, variable := amountStats(g.amounts)
		monthly := recurringMonthlyEquivalent(typical, freq)

		result.Items = append(result.Items, dto.RecurringItem{
			Merchant:          name,
			Frequency:         freq,
			TypicalAmount:     typical,
			AmountIsVariable:  variable,
			Currency:          g.currency,
			OccurrenceCount:   len(g.dates),
			LastDate:          g.dates[len(g.dates)-1],
			MonthlyEquivalent: monthly,
		})
		totalMonthly += monthly
		if currency == "" && g.currency != "" {
			currency = g.currency
		}
	}

	result.TotalMonthlyEquivalent = totalMonthly
	result.Currency = currency
	return result, nil
}

// computeGaps returns the gap in days between each consecutive pair of sorted YYYY-MM-DD dates,
// skipping same-day duplicates.
func computeGaps(sortedDates []string) ([]int, error) {
	gaps := make([]int, 0, len(sortedDates)-1)
	for i := 1; i < len(sortedDates); i++ {
		prev, err := time.Parse("2006-01-02", sortedDates[i-1])
		if err != nil {
			return nil, err
		}
		curr, err := time.Parse("2006-01-02", sortedDates[i])
		if err != nil {
			return nil, err
		}
		if gap := int(curr.Sub(prev).Hours() / 24); gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	return gaps, nil
}

// medianInt returns the median value of a non-empty slice of ints.
func medianInt(vals []int) int {
	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

// amountStats returns the median amount and whether the spread exceeds 10% of the median.
func amountStats(amounts []float64) (median float64, variable bool) {
	sorted := make([]float64, len(amounts))
	copy(sorted, amounts)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}
	if median > 0 {
		variable = (sorted[n-1]-sorted[0])/median > 0.10
	}
	return
}

// classifyFrequency maps a median gap in days to a frequency label, or "" if unrecognised.
func classifyFrequency(medianGap int) string {
	switch {
	case medianGap >= 5 && medianGap <= 9:
		return "weekly"
	case medianGap >= 10 && medianGap <= 18:
		return "biweekly"
	case medianGap >= 25 && medianGap <= 35:
		return "monthly"
	case medianGap >= 80 && medianGap <= 100:
		return "quarterly"
	default:
		return ""
	}
}

// recurringMonthlyEquivalent normalises an amount to a monthly cost for a given frequency.
func recurringMonthlyEquivalent(amount float64, frequency string) float64 {
	switch frequency {
	case "weekly":
		// 52 weeks / 12 months = 4.33 recurring charges per month.
		return amount * 4.33
	case "biweekly":
		// 26 biweekly periods / 12 months = 2.17 recurring charges per month.
		return amount * 2.17
	case "monthly":
		// Already monthly, so no adjustment.
		return amount
	case "quarterly":
		// 1 quarterly charge every 3 months, so divide by 3 for monthly equivalent.
		return amount / 3
	default:
		return 0
	}
}

// maBucket holds accumulated spend totals for a single moving-average period bucket.
type maBucket struct {
	total float64
	count int
}

func (s *analyticsService) GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error) {
	result := dto.AnalyticsMovingAverageResult{
		Granularity: args.Granularity,
		Scope:       args.Scope,
		From:        args.DateFrom,
		To:          args.DateTo,
	}

	if err := validateGranularity(args.Granularity); err != nil {
		return result, err
	}
	if err := validateScope(args.Scope); err != nil {
		return result, err
	}

	from, err := time.Parse("2006-01-02", args.DateFrom)
	if err != nil {
		return result, errs.NewValidationError("invalid dateFrom: " + args.DateFrom)
	}
	to, err := time.Parse("2006-01-02", args.DateTo)
	if err != nil {
		return result, errs.NewValidationError("invalid dateTo: " + args.DateTo)
	}
	daysAnalyzed := int(to.Sub(from).Hours()/24) + 1
	if daysAnalyzed < 1 {
		daysAnalyzed = 1
	}
	result.DaysAnalyzed = daysAnalyzed
	units := maUnits(args.Granularity, daysAnalyzed)

	overallSeries := map[string]*maBucket{}
	var overallTotal float64
	var overallCount int
	var currency string

	scopeSeries := map[string]map[string]*maBucket{}
	scopeTotals := map[string]float64{}
	scopeCounts := map[string]int{}

	pending := false
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    &pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   &args.DateFrom,
		DateTo:     &args.DateTo,
	}, func(tx *models.Transaction) error {
		pk, err := maPeriodKey(tx.Date, args.Granularity)
		if err != nil {
			return err
		}

		if _, ok := overallSeries[pk]; !ok {
			overallSeries[pk] = &maBucket{}
		}
		overallSeries[pk].total += tx.Amount
		overallSeries[pk].count++
		overallTotal += tx.Amount
		overallCount++
		if currency == "" && tx.Currency != "" {
			currency = tx.Currency
		}

		if args.Scope != "overall" {
			sk := maScopeKey(tx, args.Scope)
			if sk != "" {
				if _, ok := scopeSeries[sk]; !ok {
					scopeSeries[sk] = map[string]*maBucket{}
				}
				if _, ok := scopeSeries[sk][pk]; !ok {
					scopeSeries[sk][pk] = &maBucket{}
				}
				scopeSeries[sk][pk].total += tx.Amount
				scopeSeries[sk][pk].count++
				scopeTotals[sk] += tx.Amount
				scopeCounts[sk]++
			}
		}

		return nil
	}); err != nil {
		return result, err
	}

	if units > 0 {
		result.AveragePerUnit = overallTotal / units
	}
	result.TransactionCount = overallCount
	result.Currency = currency
	result.Series = buildMASeries(overallSeries)

	if args.Scope != "overall" {
		items := make([]dto.MovingAverageItem, 0, len(scopeSeries))
		for sk, periods := range scopeSeries {
			var avg float64
			if units > 0 {
				avg = scopeTotals[sk] / units
			}
			items = append(items, dto.MovingAverageItem{
				Key:              sk,
				AveragePerUnit:   avg,
				TransactionCount: scopeCounts[sk],
				Series:           buildMASeries(periods),
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
		result.Items = items
	}

	return result, nil
}

// buildMASeries converts a period-keyed bucket map into a sorted slice of data points.
func buildMASeries(buckets map[string]*maBucket) []dto.MovingAverageDataPoint {
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	series := make([]dto.MovingAverageDataPoint, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		series = append(series, dto.MovingAverageDataPoint{
			Period:           k,
			Total:            b.total,
			TransactionCount: b.count,
		})
	}
	return series
}

// maUnits returns the number of time units in the window for the given granularity.
func maUnits(granularity string, daysAnalyzed int) float64 {
	switch granularity {
	case "day":
		return float64(daysAnalyzed)
	case "week":
		return float64(daysAnalyzed) / 7.0
	case "month":
		return float64(daysAnalyzed) / 30.0
	default:
		return 0
	}
}

// maPeriodKey maps a YYYY-MM-DD date to the appropriate bucket key for the granularity.
func maPeriodKey(date, granularity string) (string, error) {
	switch granularity {
	case "day":
		return date, nil
	case "month":
		if len(date) < 7 {
			return "", errs.NewValidationError("invalid date: " + date)
		}
		return date[:7], nil
	case "week":
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return "", err
		}
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week), nil
	default:
		return "", errs.NewValidationError("unknown granularity: " + granularity)
	}
}

// maScopeKey extracts the grouping key from a transaction for the given scope.
func maScopeKey(tx *models.Transaction, scope string) string {
	switch scope {
	case "category":
		return tx.PFCPrimary
	case "merchant":
		return tx.Name
	default:
		return ""
	}
}

func validateGranularity(g string) error {
	switch g {
	case "day", "week", "month":
		return nil
	default:
		return errs.NewValidationError("granularity must be day, week, or month")
	}
}

func validateScope(scope string) error {
	switch scope {
	case "overall", "category", "merchant":
		return nil
	default:
		return errs.NewValidationError("scope must be overall, category, or merchant")
	}
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
