package dto

import (
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

const (
	WidgetTypeTopSpenders            = "topSpenders"
	WidgetTypeSpendingTrend          = "spendingTrend"
	WidgetTypePeriodComparison       = "periodComparison"
	WidgetTypeLargestTransactions    = "largestTransactions"
	WidgetTypeRecurringSubscriptions = "recurringSubscriptions"
)

const (
	VisPie     = "pie"
	VisBar     = "bar"
	VisList    = "list"
	VisLine    = "line"
	VisTable   = "table"
	VisSummary = "summary"
)

const (
	DateRangeThisMonth   = "thisMonth"
	DateRangeLastMonth   = "lastMonth"
	DateRangeThisQuarter = "thisQuarter"
	DateRangeLastQuarter = "lastQuarter"
	DateRangeThisYear    = "thisYear"
	DateRangeLastYear    = "lastYear"
)

const (
	Window7Day  = "7day"
	Window30Day = "30day"
	Window60Day = "60day"
	Window90Day = "90day"
)

const (
	PeriodMonthOverMonth     = "monthOverMonth"
	PeriodWeekOverWeek       = "weekOverWeek"
	PeriodQuarterOverQuarter = "quarterOverQuarter"
	PeriodYearOverYear       = "yearOverYear"
)

const (
	DimensionCategory = "category"
	DimensionMerchant = "merchant"
	DimensionOverall  = "overall"
)

type CreateWidgetRequest struct {
	Type          string              `json:"type"`
	Visualization string              `json:"visualization"`
	Config        models.WidgetConfig `json:"config"`
}

type UpdateWidgetConfigRequest struct {
	Visualization string              `json:"visualization"`
	Config        models.WidgetConfig `json:"config"`
}

type ReorderWidgetItem struct {
	WidgetID string `json:"widgetId"`
	Position int    `json:"position"`
}

type ReorderWidgetsRequest struct {
	WidgetOrder []ReorderWidgetItem `json:"widgetOrder"`
}

type WidgetDataResponse struct {
	WidgetID    string    `json:"widgetId"`
	Data        any       `json:"data"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// TopSpendersData is returned for topSpenders widgets (categories or merchants).
type TopSpendersData struct {
	Dimension        string        `json:"dimension"`
	TotalAmountMinor int64         `json:"totalAmountMinor"`
	Currency         string        `json:"currency"`
	From             string        `json:"from"`
	To               string        `json:"to"`
	Items            []SpenderItem `json:"items"`
}

type SpenderItem struct {
	Name        string  `json:"name"`
	AmountMinor int64   `json:"amountMinor"`
	Percent     float64 `json:"percent"`
	Count       int     `json:"count"`
}

// PeriodComparisonWidgetData is returned for periodComparison widgets.
type PeriodComparisonWidgetData struct {
	Current  PeriodDataItem `json:"current"`
	Previous PeriodDataItem `json:"previous"`
	Change   ChangeDataItem `json:"change"`
}

type PeriodDataItem struct {
	AmountMinor int64  `json:"amountMinor"`
	Count       int    `json:"count"`
	Currency    string `json:"currency"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type ChangeDataItem struct {
	AmountMinor int64    `json:"amountMinor"`
	Percent     *float64 `json:"percent"`
	Count       int      `json:"count"`
}

// LargestTransactionsData is returned for largestTransactions widgets.
// Each item includes both merchant and category so the client can display either.
type LargestTransactionsData struct {
	Transactions []TransactionWidgetItem `json:"transactions"`
}

type TransactionWidgetItem struct {
	TransactionID string `json:"transactionId"`
	Date          string `json:"date"`
	Merchant      string `json:"merchant"`
	AmountMinor   int64  `json:"amountMinor"`
	Category      string `json:"category"`
}

// RecurringSubscriptionsData is returned for recurringSubscriptions widgets.
type RecurringSubscriptionsData struct {
	Subscriptions     []SubscriptionItem `json:"subscriptions"`
	TotalMonthlyMinor int64              `json:"totalMonthlyMinor"`
	Currency          string             `json:"currency"`
}

type SubscriptionItem struct {
	Merchant          string `json:"merchant"`
	AmountMinor       int64  `json:"amountMinor"`
	Frequency         string `json:"frequency"`
	MonthlyEquivMinor int64  `json:"monthlyEquivMinor"`
	Variable          bool   `json:"variable"`
}
