package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type dashboardStore interface {
	Create(ctx context.Context, uid string, w *models.Widget) error
	Get(ctx context.Context, uid, widgetID string) (*models.Widget, error)
	List(ctx context.Context, uid string) ([]*models.Widget, error)
	Update(ctx context.Context, uid string, w *models.Widget) error
	Delete(ctx context.Context, uid, widgetID string) error
	Count(ctx context.Context, uid string) (int, error)
	BulkUpdatePositions(ctx context.Context, uid string, positions map[string]int) error
}

type dashboardAnalytics interface {
	GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error)
	GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error)
	GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error)
	GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error)
}

type dashboardTransactions interface {
	ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error)
}

type dashboardService struct {
	store        dashboardStore
	analytics    dashboardAnalytics
	transactions dashboardTransactions
}

func NewDashboardService(store dashboardStore, analytics dashboardAnalytics, transactions dashboardTransactions) *dashboardService {
	return &dashboardService{store: store, analytics: analytics, transactions: transactions}
}

func (s *dashboardService) GetDashboard(ctx context.Context, uid string) ([]*models.Widget, error) {
	return s.store.List(ctx, uid)
}

func (s *dashboardService) AddWidget(ctx context.Context, uid string, req dto.CreateWidgetRequest) (*models.Widget, error) {
	if err := validateWidgetType(req.Type); err != nil {
		return nil, err
	}
	if err := validateVisualization(req.Type, req.Visualization); err != nil {
		return nil, err
	}
	req.Config = applyDefaults(req.Type, req.Config)
	if err := validateConfig(req.Type, req.Config); err != nil {
		return nil, err
	}
	count, err := s.store.Count(ctx, uid)
	if err != nil {
		return nil, err
	}
	w := &models.Widget{
		WidgetID:      uuid.New().String(),
		Type:          req.Type,
		Visualization: req.Visualization,
		Position:      count + 1,
		Config:        req.Config,
	}
	if err := s.store.Create(ctx, uid, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *dashboardService) UpdateWidgetConfig(ctx context.Context, uid, widgetID string, req dto.UpdateWidgetConfigRequest) (*models.Widget, error) {
	w, err := s.store.Get(ctx, uid, widgetID)
	if err != nil {
		return nil, err
	}
	if err := validateVisualization(w.Type, req.Visualization); err != nil {
		return nil, err
	}
	if err := validateConfig(w.Type, req.Config); err != nil {
		return nil, err
	}
	w.Visualization = req.Visualization
	w.Config = req.Config
	if err := s.store.Update(ctx, uid, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *dashboardService) ReorderWidgets(ctx context.Context, uid string, req dto.ReorderWidgetsRequest) error {
	positions := make(map[string]int, len(req.WidgetOrder))
	for _, item := range req.WidgetOrder {
		positions[item.WidgetID] = item.Position
	}
	return s.store.BulkUpdatePositions(ctx, uid, positions)
}

func (s *dashboardService) DeleteWidget(ctx context.Context, uid, widgetID string) error {
	if _, err := s.store.Get(ctx, uid, widgetID); err != nil {
		return err
	}
	return s.store.Delete(ctx, uid, widgetID)
}

func (s *dashboardService) GetWidgetData(ctx context.Context, uid, widgetID string) (dto.WidgetDataResponse, error) {
	w, err := s.store.Get(ctx, uid, widgetID)
	if err != nil {
		return dto.WidgetDataResponse{}, err
	}
	var data any
	switch w.Type {
	case dto.WidgetTypeTopSpenders:
		data, err = s.fetchTopSpenders(ctx, uid, w.Config)
	case dto.WidgetTypeSpendingTrend:
		data, err = s.fetchSpendingTrend(ctx, uid, w.Config)
	case dto.WidgetTypePeriodComparison:
		data, err = s.fetchPeriodComparison(ctx, uid, w.Config)
	case dto.WidgetTypeLargestTransactions:
		data, err = s.fetchLargestTransactions(ctx, uid, w.Config)
	case dto.WidgetTypeRecurringSubscriptions:
		data, err = s.fetchRecurringSubscriptions(ctx, uid, w.Config)
	default:
		return dto.WidgetDataResponse{}, errs.NewValidationError("unknown widget type: " + w.Type)
	}
	if err != nil {
		return dto.WidgetDataResponse{}, err
	}
	return dto.WidgetDataResponse{
		WidgetID:    widgetID,
		Data:        data,
		LastUpdated: clock.Now(ctx),
	}, nil
}

func (s *dashboardService) fetchTopSpenders(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.TopSpendersData, error) {
	from, to, err := resolveDateRange(*cfg.DateRange, clock.Now(ctx))
	if err != nil {
		return dto.TopSpendersData{}, err
	}
	result, err := s.analytics.GetTopN(ctx, uid, dto.AnalyticsTopNArgs{
		Dimension:  cfg.Dimension,
		Direction:  "top",
		Limit:      cfg.Limit,
		PFCPrimary: helpers.OptString(cfg.Category),
		AccountID:  helpers.OptString(cfg.AccountID),
		DateFrom:   from,
		DateTo:     to,
	})
	if err != nil {
		return dto.TopSpendersData{}, err
	}
	items := make([]dto.SpenderItem, len(result.Items))
	for i, it := range result.Items {
		items[i] = dto.SpenderItem{
			Name:    it.Key,
			Amount:  it.Total,
			Percent: it.Percentage,
			Count:   it.Count,
		}
	}
	return dto.TopSpendersData{
		Dimension:   result.Dimension,
		TotalAmount: result.TotalSpend,
		Currency:    result.Currency,
		From:        result.From,
		To:          result.To,
		Items:       items,
	}, nil
}

func (s *dashboardService) fetchSpendingTrend(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.AnalyticsMovingAverageResult, error) {
	var from, to string
	var err error
	if cfg.DateRange != nil {
		from, to, err = resolveDateRange(*cfg.DateRange, clock.Now(ctx))
	} else {
		from, to, err = resolveWindow(cfg.Window, clock.Now(ctx))
	}
	if err != nil {
		return dto.AnalyticsMovingAverageResult{}, err
	}
	return s.analytics.GetMovingAverage(ctx, uid, dto.AnalyticsMovingAverageArgs{
		Granularity: "day",
		Scope:       cfg.Dimension,
		PFCPrimary:  helpers.OptString(cfg.Category),
		AccountID:   helpers.OptString(cfg.AccountID),
		DateFrom:    from,
		DateTo:      to,
	})
}

func (s *dashboardService) fetchPeriodComparison(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.PeriodComparisonWidgetData, error) {
	var currFrom, currTo, prevFrom, prevTo string
	var err error
	if cfg.CurrentRange != nil && cfg.PreviousRange != nil {
		currFrom, currTo = cfg.CurrentRange.StartDate, cfg.CurrentRange.EndDate
		prevFrom, prevTo = cfg.PreviousRange.StartDate, cfg.PreviousRange.EndDate
	} else {
		currFrom, currTo, prevFrom, prevTo, err = resolvePeriodPreset(cfg.Preset, clock.Now(ctx))
		if err != nil {
			return dto.PeriodComparisonWidgetData{}, err
		}
	}
	result, err := s.analytics.GetPeriodComparison(ctx, uid, dto.AnalyticsPeriodComparisonArgs{
		AccountID:    helpers.OptString(cfg.AccountID),
		CurrentFrom:  currFrom,
		CurrentTo:    currTo,
		PreviousFrom: prevFrom,
		PreviousTo:   prevTo,
	})
	if err != nil {
		return dto.PeriodComparisonWidgetData{}, err
	}
	return dto.PeriodComparisonWidgetData{
		Current: dto.PeriodDataItem{
			Amount:   result.Current.Total,
			Count:    result.Current.Count,
			Currency: result.Current.Currency,
			From:     result.Current.From,
			To:       result.Current.To,
		},
		Previous: dto.PeriodDataItem{
			Amount:   result.Previous.Total,
			Count:    result.Previous.Count,
			Currency: result.Previous.Currency,
			From:     result.Previous.From,
			To:       result.Previous.To,
		},
		Change: dto.ChangeDataItem{
			Amount:  result.Change.AbsoluteChange,
			Percent: result.Change.PercentageChange,
			Count:   result.Change.CountChange,
		},
	}, nil
}

func (s *dashboardService) fetchLargestTransactions(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.LargestTransactionsData, error) {
	from, to, err := resolveDateRange(*cfg.DateRange, clock.Now(ctx))
	if err != nil {
		return dto.LargestTransactionsData{}, err
	}
	result, err := s.transactions.ListTransactions(ctx, uid, dto.TransactionListArgs{
		Pending:      helpers.Ptr(false),
		PFCPrimaries: helpers.PrimarySlice(helpers.OptString(cfg.Category)),
		AccountID:    helpers.OptString(cfg.AccountID),
		DateFrom:     &from,
		DateTo:       &to,
		OrderBy:      "amount",
		Desc:         true,
		Limit:        cfg.Limit,
	})
	if err != nil {
		return dto.LargestTransactionsData{}, err
	}
	items := make([]dto.TransactionWidgetItem, len(result.Transactions))
	for i, tx := range result.Transactions {
		items[i] = dto.TransactionWidgetItem{
			TransactionID: tx.TransactionID,
			Date:          tx.Date,
			Merchant:      tx.Name,
			Amount:        tx.Amount,
			Category:      tx.PFCPrimary,
		}
	}
	return dto.LargestTransactionsData{Transactions: items}, nil
}

func (s *dashboardService) fetchRecurringSubscriptions(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.RecurringSubscriptionsData, error) {
	var from, to string
	if cfg.DateRange != nil {
		var err error
		from, to, err = resolveDateRange(*cfg.DateRange, clock.Now(ctx))
		if err != nil {
			return dto.RecurringSubscriptionsData{}, err
		}
	} else {
		now := clock.Now(ctx)
		from = helpers.FormatDate(now.AddDate(0, -6, 0))
		to = helpers.FormatDate(now)
	}
	result, err := s.analytics.GetRecurringTransactions(ctx, uid, dto.AnalyticsRecurringArgs{
		AccountID: helpers.OptString(cfg.AccountID),
		DateFrom:  from,
		DateTo:    to,
	})
	if err != nil {
		return dto.RecurringSubscriptionsData{}, err
	}
	items := make([]dto.SubscriptionItem, len(result.Items))
	for i, it := range result.Items {
		items[i] = dto.SubscriptionItem{
			Merchant:     it.Merchant,
			Amount:       it.TypicalAmount,
			Frequency:    it.Frequency,
			MonthlyEquiv: it.MonthlyEquivalent,
			Variable:     it.AmountIsVariable,
		}
	}
	return dto.RecurringSubscriptionsData{
		Subscriptions: items,
		TotalMonthly:  result.TotalMonthlyEquivalent,
		Currency:      result.Currency,
	}, nil
}

func validateWidgetType(t string) error {
	switch t {
	case dto.WidgetTypeTopSpenders, dto.WidgetTypeSpendingTrend,
		dto.WidgetTypePeriodComparison, dto.WidgetTypeLargestTransactions,
		dto.WidgetTypeRecurringSubscriptions:
		return nil
	}
	return errs.NewValidationError("unknown widget type: " + t)
}

var validVisualizations = map[string][]string{
	dto.WidgetTypeTopSpenders:            {dto.VisPie, dto.VisBar, dto.VisList},
	dto.WidgetTypeSpendingTrend:          {dto.VisLine, dto.VisBar},
	dto.WidgetTypePeriodComparison:       {dto.VisSummary, dto.VisBar},
	dto.WidgetTypeLargestTransactions:    {dto.VisList, dto.VisTable},
	dto.WidgetTypeRecurringSubscriptions: {dto.VisList, dto.VisTable},
}

func validateVisualization(widgetType, vis string) error {
	for _, v := range validVisualizations[widgetType] {
		if v == vis {
			return nil
		}
	}
	return errs.NewValidationError(fmt.Sprintf("visualization %q is not valid for widget type %q", vis, widgetType))
}

func applyDefaults(widgetType string, cfg models.WidgetConfig) models.WidgetConfig {
	switch widgetType {
	case dto.WidgetTypeTopSpenders:
		if cfg.Limit == 0 {
			cfg.Limit = 10
		}
	case dto.WidgetTypeSpendingTrend:
		if cfg.Window == "" && cfg.DateRange == nil {
			cfg.Window = dto.Window30Day
		}
	case dto.WidgetTypeLargestTransactions:
		if cfg.Limit == 0 {
			cfg.Limit = 10
		}
	}
	return cfg
}

func validateConfig(widgetType string, cfg models.WidgetConfig) error {
	switch widgetType {
	case dto.WidgetTypeTopSpenders:
		if cfg.DateRange == nil {
			return errs.NewValidationError("config.dateRange is required for topSpenders")
		}
		if err := validateDateRange(*cfg.DateRange); err != nil {
			return err
		}
		if cfg.Dimension != dto.DimensionCategory && cfg.Dimension != dto.DimensionMerchant {
			return errs.NewValidationError(`config.dimension must be "category" or "merchant" for topSpenders`)
		}
		if cfg.Limit < 3 || cfg.Limit > 20 {
			return errs.NewValidationError("config.limit must be between 3 and 20 for topSpenders")
		}

	case dto.WidgetTypeSpendingTrend:
		if cfg.Window != "" && cfg.DateRange != nil {
			return errs.NewValidationError("config.spendingTrend: set window or dateRange, not both")
		}
		if cfg.Window != "" {
			switch cfg.Window {
			case dto.Window7Day, dto.Window30Day, dto.Window60Day, dto.Window90Day:
			default:
				return errs.NewValidationError("config.window must be one of: 7day, 30day, 60day, 90day")
			}
		}
		if cfg.DateRange != nil {
			if err := validateDateRange(*cfg.DateRange); err != nil {
				return err
			}
		}
		switch cfg.Dimension {
		case dto.DimensionOverall, dto.DimensionCategory, dto.DimensionMerchant:
		default:
			return errs.NewValidationError("config.dimension must be one of: overall, category, merchant")
		}

	case dto.WidgetTypePeriodComparison:
		hasPreset := cfg.Preset != ""
		hasCustom := cfg.CurrentRange != nil || cfg.PreviousRange != nil
		if hasPreset && hasCustom {
			return errs.NewValidationError("config.periodComparison: set preset or custom ranges (currentRange/previousRange), not both")
		}
		if hasPreset {
			switch cfg.Preset {
			case dto.PeriodMonthOverMonth, dto.PeriodWeekOverWeek,
				dto.PeriodQuarterOverQuarter, dto.PeriodYearOverYear:
			default:
				return errs.NewValidationError("config.preset must be one of: monthOverMonth, weekOverWeek, quarterOverQuarter, yearOverYear")
			}
		} else {
			if cfg.CurrentRange == nil || cfg.PreviousRange == nil {
				return errs.NewValidationError("config.periodComparison: provide either preset or both currentRange and previousRange")
			}
			if err := validateExplicitDateRange(*cfg.CurrentRange); err != nil {
				return fmt.Errorf("config.currentRange: %w", err)
			}
			if err := validateExplicitDateRange(*cfg.PreviousRange); err != nil {
				return fmt.Errorf("config.previousRange: %w", err)
			}
		}

	case dto.WidgetTypeLargestTransactions:
		if cfg.DateRange == nil {
			return errs.NewValidationError("config.dateRange is required for largestTransactions")
		}
		if err := validateDateRange(*cfg.DateRange); err != nil {
			return err
		}
		if cfg.Limit < 5 || cfg.Limit > 20 {
			return errs.NewValidationError("config.limit must be between 5 and 20 for largestTransactions")
		}

	case dto.WidgetTypeRecurringSubscriptions:
		if cfg.DateRange != nil {
			if err := validateDateRange(*cfg.DateRange); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDateRange(dr models.DateRangeConfig) error {
	if dr.Preset != "" {
		switch dr.Preset {
		case dto.DateRangeThisMonth, dto.DateRangeLastMonth,
			dto.DateRangeThisQuarter, dto.DateRangeLastQuarter,
			dto.DateRangeThisYear, dto.DateRangeLastYear:
		default:
			return errs.NewValidationError("unknown date range preset: " + dr.Preset)
		}
		return nil
	}
	if dr.StartDate == "" || dr.EndDate == "" {
		return errs.NewValidationError("config.dateRange requires either a preset or both startDate and endDate")
	}
	return nil
}

func validateExplicitDateRange(r models.ExplicitDateRange) error {
	if r.StartDate == "" || r.EndDate == "" {
		return errs.NewValidationError("startDate and endDate are required")
	}
	start, err := helpers.ParseDate(r.StartDate)
	if err != nil {
		return errs.NewValidationError("startDate must be in YYYY-MM-DD format")
	}
	end, err := helpers.ParseDate(r.EndDate)
	if err != nil {
		return errs.NewValidationError("endDate must be in YYYY-MM-DD format")
	}
	if !end.After(start) {
		return errs.NewValidationError("endDate must be after startDate")
	}
	return nil
}

func resolveDateRange(dr models.DateRangeConfig, now time.Time) (from, to string, err error) {
	if dr.Preset != "" {
		return resolvePreset(dr.Preset, now)
	}
	if dr.StartDate == "" || dr.EndDate == "" {
		return "", "", errs.NewValidationError("config.dateRange requires either a preset or both startDate and endDate")
	}
	return dr.StartDate, dr.EndDate, nil
}

func resolvePreset(preset string, now time.Time) (from, to string, err error) {
	today := helpers.FormatDate(now)
	switch preset {
	case dto.DateRangeThisMonth:
		return helpers.FormatDate(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())), today, nil
	case dto.DateRangeLastMonth:
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfPrev := firstOfMonth.AddDate(0, 0, -1)
		firstOfPrev := time.Date(lastOfPrev.Year(), lastOfPrev.Month(), 1, 0, 0, 0, 0, now.Location())
		return helpers.FormatDate(firstOfPrev), helpers.FormatDate(lastOfPrev), nil
	case dto.DateRangeThisQuarter:
		return helpers.FormatDate(firstOfQuarter(now)), today, nil
	case dto.DateRangeLastQuarter:
		f, l := prevQuarter(now)
		return helpers.FormatDate(f), helpers.FormatDate(l), nil
	case dto.DateRangeThisYear:
		return helpers.FormatDate(time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())), today, nil
	case dto.DateRangeLastYear:
		return helpers.FormatDate(time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())),
			helpers.FormatDate(time.Date(now.Year()-1, 12, 31, 0, 0, 0, 0, now.Location())), nil
	}
	return "", "", errs.NewValidationError("unknown date range preset: " + preset)
}

func resolveWindow(window string, now time.Time) (from, to string, err error) {
	today := helpers.FormatDate(now)
	switch window {
	case dto.Window7Day:
		return helpers.FormatDate(now.AddDate(0, 0, -7)), today, nil
	case dto.Window30Day:
		return helpers.FormatDate(now.AddDate(0, 0, -30)), today, nil
	case dto.Window60Day:
		return helpers.FormatDate(now.AddDate(0, 0, -60)), today, nil
	case dto.Window90Day:
		return helpers.FormatDate(now.AddDate(0, 0, -90)), today, nil
	}
	return "", "", errs.NewValidationError("unknown window: " + window)
}

func resolvePeriodPreset(preset string, now time.Time) (currFrom, currTo, prevFrom, prevTo string, err error) {
	today := helpers.FormatDate(now)
	switch preset {
	case dto.PeriodMonthOverMonth:
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfPrev := firstOfMonth.AddDate(0, 0, -1)
		firstOfPrev := time.Date(lastOfPrev.Year(), lastOfPrev.Month(), 1, 0, 0, 0, 0, now.Location())
		return helpers.FormatDate(firstOfMonth), today,
			helpers.FormatDate(firstOfPrev), helpers.FormatDate(lastOfPrev), nil
	case dto.PeriodWeekOverWeek:
		thisMonday := mondayOfWeek(now)
		prevMonday := thisMonday.AddDate(0, 0, -7)
		prevSunday := thisMonday.AddDate(0, 0, -1)
		return helpers.FormatDate(thisMonday), today,
			helpers.FormatDate(prevMonday), helpers.FormatDate(prevSunday), nil
	case dto.PeriodQuarterOverQuarter:
		f := firstOfQuarter(now)
		prevF, prevL := prevQuarter(now)
		return helpers.FormatDate(f), today,
			helpers.FormatDate(prevF), helpers.FormatDate(prevL), nil
	case dto.PeriodYearOverYear:
		thisJan1 := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		prevJan1 := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		prevSameDay := now.AddDate(-1, 0, 0)
		return helpers.FormatDate(thisJan1), today,
			helpers.FormatDate(prevJan1), helpers.FormatDate(prevSameDay), nil
	}
	return "", "", "", "", errs.NewValidationError("unknown period preset: " + preset)
}

func firstOfQuarter(t time.Time) time.Time {
	m := int(t.Month())
	qStart := ((m-1)/3)*3 + 1
	return time.Date(t.Year(), time.Month(qStart), 1, 0, 0, 0, 0, t.Location())
}

func prevQuarter(t time.Time) (first, last time.Time) {
	thisFirst := firstOfQuarter(t)
	last = thisFirst.AddDate(0, 0, -1)
	first = firstOfQuarter(last)
	return
}

func mondayOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // ISO: Sunday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}
