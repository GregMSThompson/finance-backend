package services

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
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

	var total int64
	var currency string
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:      args.Pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		Merchant:     args.Merchant,
		DateFrom:     args.DateFrom,
		DateTo:       args.DateTo,
	}, func(tx *models.Transaction) error {
		// Income and transfers aren't spending, so they're left out of the total.
		// A caller that explicitly filtered to one of those categories (non-nil
		// PFCPrimary) gets exactly what they asked for.
		if args.PFCPrimary == nil && taxonomy.IsNonSpendCategory(tx.PFCPrimary) {
			return nil
		}
		total += tx.AmountMinor
		if currency == "" && tx.Currency != "" {
			currency = tx.Currency
		}
		return nil
	}); err != nil {
		return result, err
	}

	result.TotalMinor = total
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
		Pending:      args.Pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		DateFrom:     args.DateFrom,
		DateTo:       args.DateTo,
	}, args.GroupBy)
	if err != nil {
		return result, err
	}

	result.Currency = data.currency
	result.Items = mapBreakdownItems(data.items)
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
		Pending:      args.Pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		Merchant:     args.Merchant,
		DateFrom:     &args.CurrentFrom,
		DateTo:       &args.CurrentTo,
	}
	previousQuery := dto.TransactionQuery{
		Pending:      args.Pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		Merchant:     args.Merchant,
		DateFrom:     &args.PreviousFrom,
		DateTo:       &args.PreviousTo,
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
		TotalMinor: currentData.total,
		Count:      currentData.count,
		Currency:   currentData.currency,
		From:       args.CurrentFrom,
		To:         args.CurrentTo,
		Items:      currentItems,
	}
	result.Previous = dto.PeriodSummary{
		TotalMinor: previousData.total,
		Count:      previousData.count,
		Currency:   previousData.currency,
		From:       args.PreviousFrom,
		To:         args.PreviousTo,
		Items:      previousItems,
	}
	result.Change = buildChange(currentData, previousData, args.GroupBy)

	return result, nil
}

// periodData holds the accumulated totals for a single query period. total is in
// integer minor units.
type periodData struct {
	total    int64
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
		data.total += tx.AmountMinor
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
				item.TotalMinor += tx.AmountMinor
				item.Count++
			}
		}
		return nil
	})
	return data, err
}

func buildChange(current, previous periodData, groupBy string) dto.PeriodChange {
	change := dto.PeriodChange{
		AbsoluteChangeMinor: current.total - previous.total,
		PercentageChange:    percentageChange(current.total, previous.total),
		CountChange:         current.count - previous.count,
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
			var currTotal int64
			var currCount int
			var prevTotal int64
			var prevCount int

			if item := current.items[key]; item != nil {
				currTotal = item.TotalMinor
				currCount = item.Count
			}
			if item := previous.items[key]; item != nil {
				prevTotal = item.TotalMinor
				prevCount = item.Count
			}

			change.Items = append(change.Items, dto.BreakdownItemChange{
				Key:                 key,
				AbsoluteChangeMinor: currTotal - prevTotal,
				PercentageChange:    percentageChange(currTotal, prevTotal),
				CountChange:         currCount - prevCount,
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
		dates      []string
		amounts    []int64
		currency   string
		lastDate   string
		lastAmount int64
	}

	pending := false
	groups := map[string]*merchantGroup{}

	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:   &pending,
		AccountID: args.AccountID,
		DateFrom:  &args.DateFrom,
		DateTo:    &args.DateTo,
	}, func(tx *models.Transaction) error {
		g, ok := groups[tx.Name]
		if !ok {
			g = &merchantGroup{}
			groups[tx.Name] = g
		}
		g.dates = append(g.dates, tx.Date)
		g.amounts = append(g.amounts, tx.AmountMinor)
		if g.currency == "" && tx.Currency != "" {
			g.currency = tx.Currency
		}
		if tx.Date >= g.lastDate {
			g.lastDate = tx.Date
			g.lastAmount = tx.AmountMinor
		}
		return nil
	}); err != nil {
		return result, err
	}

	var totalMonthly int64
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
			Merchant:               name,
			Frequency:              freq,
			TypicalAmountMinor:     typical,
			AmountIsVariable:       variable,
			Currency:               g.currency,
			OccurrenceCount:        len(g.dates),
			LastDate:               g.lastDate,
			LastAmountMinor:        g.lastAmount,
			MonthlyEquivalentMinor: monthly,
		})
		totalMonthly += monthly
		if currency == "" && g.currency != "" {
			currency = g.currency
		}
	}

	result.TotalMonthlyEquivalentMinor = totalMonthly
	result.Currency = currency
	return result, nil
}

// computeGaps returns the gap in days between each consecutive pair of sorted YYYY-MM-DD dates,
// skipping same-day duplicates.
func computeGaps(sortedDates []string) ([]int, error) {
	gaps := make([]int, 0, len(sortedDates)-1)
	for i := 1; i < len(sortedDates); i++ {
		prev, err := helpers.ParseDate(sortedDates[i-1])
		if err != nil {
			return nil, err
		}
		curr, err := helpers.ParseDate(sortedDates[i])
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

// amountStats returns the median amount (in minor units) and whether the spread
// exceeds 10% of the median. The even-count median is rounded to the nearest
// minor unit so the result stays an integer amount.
func amountStats(amounts []int64) (median int64, variable bool) {
	sorted := slices.Clone(amounts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 0 {
		median = int64(math.Round(float64(sorted[n/2-1]+sorted[n/2]) / 2))
	} else {
		median = sorted[n/2]
	}
	if median > 0 {
		variable = float64(sorted[n-1]-sorted[0])/float64(median) > 0.10
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

// recurringMonthlyEquivalent normalises an amount (in minor units) to a monthly
// cost for a given frequency, rounding back to the nearest minor unit.
func recurringMonthlyEquivalent(amount int64, frequency string) int64 {
	switch frequency {
	case "weekly":
		// 52 weeks / 12 months = 4.33 recurring charges per month.
		return int64(math.Round(float64(amount) * 4.33))
	case "biweekly":
		// 26 biweekly periods / 12 months = 2.17 recurring charges per month.
		return int64(math.Round(float64(amount) * 2.17))
	case "monthly":
		// Already monthly, so no adjustment.
		return amount
	case "quarterly":
		// 1 quarterly charge every 3 months, so divide by 3 for monthly equivalent.
		return int64(math.Round(float64(amount) / 3))
	default:
		return 0
	}
}

func (s *analyticsService) GetIncomeVsExpenses(ctx context.Context, uid string, args dto.AnalyticsIncomeVsExpensesArgs) (dto.IncomeVsExpensesResult, error) {
	result := dto.IncomeVsExpensesResult{
		From: args.DateFrom,
		To:   args.DateTo,
	}

	pending := false
	var total, income int64
	var currency string

	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:   &pending,
		AccountID: args.AccountID,
		DateFrom:  &args.DateFrom,
		DateTo:    &args.DateTo,
	}, func(tx *models.Transaction) error {
		total += tx.AmountMinor
		if tx.PFCPrimary == "INCOME" {
			income += tx.AmountMinor
		}
		if currency == "" && tx.Currency != "" {
			currency = tx.Currency
		}
		return nil
	}); err != nil {
		return result, err
	}

	result.IncomeMinor = income
	result.ExpensesMinor = total - income
	result.NetMinor = income - result.ExpensesMinor
	result.Currency = currency
	return result, nil
}

func (s *analyticsService) GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error) {
	result := dto.AnalyticsTopNResult{
		Dimension: args.Dimension,
		Direction: args.Direction,
		From:      args.DateFrom,
		To:        args.DateTo,
		Items:     []dto.TopNItem{},
	}

	groupBy, err := topNGroupBy(args.Dimension)
	if err != nil {
		return result, err
	}

	pending := false
	data, err := collectPeriod(ctx, s.txs, uid, dto.TransactionQuery{
		Pending:      &pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		DateFrom:     &args.DateFrom,
		DateTo:       &args.DateTo,
	}, groupBy)
	if err != nil {
		return result, err
	}

	result.TotalSpendMinor = data.total
	result.Currency = data.currency

	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}

	items := make([]dto.TopNItem, 0, len(data.items))
	for _, item := range data.items {
		if args.MinCount > 0 && item.Count < args.MinCount {
			continue
		}
		var pct float64
		if data.total > 0 {
			pct = float64(item.TotalMinor) / float64(data.total) * 100
		}
		items = append(items, dto.TopNItem{
			Key:        item.Key,
			TotalMinor: item.TotalMinor,
			Count:      item.Count,
			Percentage: pct,
		})
	}

	desc := args.Direction != "bottom"
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalMinor != items[j].TotalMinor {
			if desc {
				return items[i].TotalMinor > items[j].TotalMinor
			}
			return items[i].TotalMinor < items[j].TotalMinor
		}
		if items[i].Count != items[j].Count {
			if desc {
				return items[i].Count > items[j].Count
			}
			return items[i].Count < items[j].Count
		}
		return items[i].Key < items[j].Key
	})

	if limit < len(items) {
		items = items[:limit]
	}
	result.Items = items
	return result, nil
}

// topNGroupBy translates a GetTopN dimension to the groupBy key used by collectPeriod.
func topNGroupBy(dimension string) (string, error) {
	switch dimension {
	case "merchant":
		return "merchant", nil
	case "category":
		return "pfcPrimary", nil
	default:
		return "", errs.NewValidationError("dimension must be merchant or category")
	}
}

// maBucket holds accumulated spend totals (in minor units) for a single
// moving-average period bucket.
type maBucket struct {
	total int64
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

	from, err := helpers.ParseDate(args.DateFrom)
	if err != nil {
		return result, errs.NewValidationError("invalid dateFrom: " + args.DateFrom)
	}
	to, err := helpers.ParseDate(args.DateTo)
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
	var overallTotal int64
	var overallCount int
	var currency string

	scopeSeries := map[string]map[string]*maBucket{}
	scopeTotals := map[string]int64{}
	scopeCounts := map[string]int{}

	pending := false
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:      &pending,
		PFCPrimaries: helpers.PrimarySlice(args.PFCPrimary),
		AccountID:    args.AccountID,
		Merchant:     args.Merchant,
		DateFrom:     &args.DateFrom,
		DateTo:       &args.DateTo,
	}, func(tx *models.Transaction) error {
		pk, err := maPeriodKey(tx.Date, args.Granularity)
		if err != nil {
			return err
		}

		if _, ok := overallSeries[pk]; !ok {
			overallSeries[pk] = &maBucket{}
		}
		overallSeries[pk].total += tx.AmountMinor
		overallSeries[pk].count++
		overallTotal += tx.AmountMinor
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
				scopeSeries[sk][pk].total += tx.AmountMinor
				scopeSeries[sk][pk].count++
				scopeTotals[sk] += tx.AmountMinor
				scopeCounts[sk]++
			}
		}

		return nil
	}); err != nil {
		return result, err
	}

	if units > 0 {
		result.AveragePerUnitMinor = float64(overallTotal) / units
	}
	result.TransactionCount = overallCount
	result.Currency = currency
	result.Series = buildMASeries(overallSeries)

	if args.Scope != "overall" {
		items := make([]dto.MovingAverageItem, 0, len(scopeSeries))
		for sk, periods := range scopeSeries {
			var avg float64
			if units > 0 {
				avg = float64(scopeTotals[sk]) / units
			}
			items = append(items, dto.MovingAverageItem{
				Key:                 sk,
				AveragePerUnitMinor: avg,
				TransactionCount:    scopeCounts[sk],
				Series:              buildMASeries(periods),
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
			TotalMinor:       b.total,
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
		t, err := helpers.ParseDate(date)
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

func percentageChange(current, previous int64) *float64 {
	if previous == 0 {
		return nil
	}
	pct := float64(current-previous) / float64(previous) * 100
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
