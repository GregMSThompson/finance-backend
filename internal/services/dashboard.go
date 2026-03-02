package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

const dashDateLayout = "2006-01-02"

// dashboardStore is the Firestore storage interface for widgets.
type dashboardStore interface {
	Create(ctx context.Context, uid string, w *models.Widget) error
	Get(ctx context.Context, uid, widgetID string) (*models.Widget, error)
	List(ctx context.Context, uid string) ([]*models.Widget, error)
	Update(ctx context.Context, uid string, w *models.Widget) error
	Delete(ctx context.Context, uid, widgetID string) error
	Count(ctx context.Context, uid string) (int, error)
	BulkUpdatePositions(ctx context.Context, uid string, positions map[string]int) error
}

// dashboardAnalytics is the analytics interface used by dashboardService.
type dashboardAnalytics interface {
	GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error)
	GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error)
	GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error)
	GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error)
	GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error)
}

type dashboardService struct {
	store     dashboardStore
	analytics dashboardAnalytics
}

func NewDashboardService(store dashboardStore, analytics dashboardAnalytics) *dashboardService {
	return &dashboardService{store: store, analytics: analytics}
}

// --- Public service methods ---

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
	if err := validateConfig(w.Type, req.Config); err != nil {
		return nil, err
	}
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
		LastUpdated: time.Now(),
	}, nil
}

// --- Private fetch methods ---

func (s *dashboardService) fetchTopSpenders(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.TopSpendersData, error) {
	from, to, err := resolveDateRange(*cfg.DateRange, time.Now())
	if err != nil {
		return dto.TopSpendersData{}, err
	}
	result, err := s.analytics.GetTopN(ctx, uid, dto.AnalyticsTopNArgs{
		Dimension:  cfg.Dimension,
		Direction:  "top",
		Limit:      cfg.Limit,
		PFCPrimary: optString(cfg.Category),
		BankID:     optString(cfg.BankID),
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
	from, to, err := resolveWindow(cfg.Window, time.Now())
	if err != nil {
		return dto.AnalyticsMovingAverageResult{}, err
	}
	return s.analytics.GetMovingAverage(ctx, uid, dto.AnalyticsMovingAverageArgs{
		Granularity: "day",
		Scope:       cfg.Dimension,
		PFCPrimary:  optString(cfg.Category),
		BankID:      optString(cfg.BankID),
		DateFrom:    from,
		DateTo:      to,
	})
}

func (s *dashboardService) fetchPeriodComparison(ctx context.Context, uid string, cfg models.WidgetConfig) (dto.PeriodComparisonWidgetData, error) {
	currFrom, currTo, prevFrom, prevTo, err := resolvePeriodPreset(cfg.Preset, time.Now())
	if err != nil {
		return dto.PeriodComparisonWidgetData{}, err
	}
	result, err := s.analytics.GetPeriodComparison(ctx, uid, dto.AnalyticsPeriodComparisonArgs{
		BankID:       optString(cfg.BankID),
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
	from, to, err := resolveDateRange(*cfg.DateRange, time.Now())
	if err != nil {
		return dto.LargestTransactionsData{}, err
	}
	result, err := s.analytics.GetTransactions(ctx, uid, dto.AnalyticsTransactionsArgs{
		PFCPrimary: optString(cfg.Category),
		BankID:     optString(cfg.BankID),
		DateFrom:   &from,
		DateTo:     &to,
		OrderBy:    "amount",
		Desc:       true,
		Limit:      cfg.Limit,
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
	now := time.Now()
	from := now.AddDate(-1, 0, 0).Format(dashDateLayout)
	to := now.Format(dashDateLayout)
	result, err := s.analytics.GetRecurringTransactions(ctx, uid, dto.AnalyticsRecurringArgs{
		BankID:   optString(cfg.BankID),
		DateFrom: from,
		DateTo:   to,
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

// --- Validation ---

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
		switch cfg.Window {
		case dto.Window7Day, dto.Window30Day, dto.Window60Day, dto.Window90Day:
		default:
			return errs.NewValidationError("config.window must be one of: 7day, 30day, 60day, 90day")
		}
		switch cfg.Dimension {
		case dto.DimensionOverall, dto.DimensionCategory, dto.DimensionMerchant:
		default:
			return errs.NewValidationError("config.dimension must be one of: overall, category, merchant")
		}

	case dto.WidgetTypePeriodComparison:
		switch cfg.Preset {
		case dto.PeriodMonthOverMonth, dto.PeriodWeekOverWeek,
			dto.PeriodQuarterOverQuarter, dto.PeriodYearOverYear:
		default:
			return errs.NewValidationError("config.preset must be one of: monthOverMonth, weekOverWeek, quarterOverQuarter, yearOverYear")
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
		// No required config fields.
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

// --- Date resolution ---

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
	today := now.Format(dashDateLayout)
	switch preset {
	case dto.DateRangeThisMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format(dashDateLayout), today, nil
	case dto.DateRangeLastMonth:
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfPrev := firstOfMonth.AddDate(0, 0, -1)
		firstOfPrev := time.Date(lastOfPrev.Year(), lastOfPrev.Month(), 1, 0, 0, 0, 0, now.Location())
		return firstOfPrev.Format(dashDateLayout), lastOfPrev.Format(dashDateLayout), nil
	case dto.DateRangeThisQuarter:
		return firstOfQuarter(now).Format(dashDateLayout), today, nil
	case dto.DateRangeLastQuarter:
		f, l := prevQuarter(now)
		return f.Format(dashDateLayout), l.Format(dashDateLayout), nil
	case dto.DateRangeThisYear:
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Format(dashDateLayout), today, nil
	case dto.DateRangeLastYear:
		return time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location()).Format(dashDateLayout),
			time.Date(now.Year()-1, 12, 31, 0, 0, 0, 0, now.Location()).Format(dashDateLayout), nil
	}
	return "", "", errs.NewValidationError("unknown date range preset: " + preset)
}

func resolveWindow(window string, now time.Time) (from, to string, err error) {
	today := now.Format(dashDateLayout)
	switch window {
	case dto.Window7Day:
		return now.AddDate(0, 0, -7).Format(dashDateLayout), today, nil
	case dto.Window30Day:
		return now.AddDate(0, 0, -30).Format(dashDateLayout), today, nil
	case dto.Window60Day:
		return now.AddDate(0, 0, -60).Format(dashDateLayout), today, nil
	case dto.Window90Day:
		return now.AddDate(0, 0, -90).Format(dashDateLayout), today, nil
	}
	return "", "", errs.NewValidationError("unknown window: " + window)
}

func resolvePeriodPreset(preset string, now time.Time) (currFrom, currTo, prevFrom, prevTo string, err error) {
	today := now.Format(dashDateLayout)
	switch preset {
	case dto.PeriodMonthOverMonth:
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfPrev := firstOfMonth.AddDate(0, 0, -1)
		firstOfPrev := time.Date(lastOfPrev.Year(), lastOfPrev.Month(), 1, 0, 0, 0, 0, now.Location())
		return firstOfMonth.Format(dashDateLayout), today,
			firstOfPrev.Format(dashDateLayout), lastOfPrev.Format(dashDateLayout), nil
	case dto.PeriodWeekOverWeek:
		thisMonday := mondayOfWeek(now)
		prevMonday := thisMonday.AddDate(0, 0, -7)
		prevSunday := thisMonday.AddDate(0, 0, -1)
		return thisMonday.Format(dashDateLayout), today,
			prevMonday.Format(dashDateLayout), prevSunday.Format(dashDateLayout), nil
	case dto.PeriodQuarterOverQuarter:
		f := firstOfQuarter(now)
		prevF, prevL := prevQuarter(now)
		return f.Format(dashDateLayout), today,
			prevF.Format(dashDateLayout), prevL.Format(dashDateLayout), nil
	case dto.PeriodYearOverYear:
		thisJan1 := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		prevJan1 := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		prevSameDay := now.AddDate(-1, 0, 0)
		return thisJan1.Format(dashDateLayout), today,
			prevJan1.Format(dashDateLayout), prevSameDay.Format(dashDateLayout), nil
	}
	return "", "", "", "", errs.NewValidationError("unknown period preset: " + preset)
}

// --- Calendar helpers ---

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

// --- Helpers ---

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
