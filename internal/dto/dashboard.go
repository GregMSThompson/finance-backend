package dto

import (
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

// Widget type constants
const (
	WidgetTypeTopSpenders            = "topSpenders"
	WidgetTypeSpendingTrend          = "spendingTrend"
	WidgetTypePeriodComparison       = "periodComparison"
	WidgetTypeLargestTransactions    = "largestTransactions"
	WidgetTypeRecurringSubscriptions = "recurringSubscriptions"
)

// Visualization constants
const (
	VisPie     = "pie"
	VisBar     = "bar"
	VisList    = "list"
	VisLine    = "line"
	VisTable   = "table"
	VisSummary = "summary"
)

// Date range presets
const (
	DateRangeThisMonth   = "thisMonth"
	DateRangeLastMonth   = "lastMonth"
	DateRangeThisQuarter = "thisQuarter"
	DateRangeLastQuarter = "lastQuarter"
	DateRangeThisYear    = "thisYear"
	DateRangeLastYear    = "lastYear"
)

// Window presets (for spendingTrend)
const (
	Window7Day  = "7day"
	Window30Day = "30day"
	Window60Day = "60day"
	Window90Day = "90day"
)

// Period comparison presets
const (
	PeriodMonthOverMonth     = "monthOverMonth"
	PeriodWeekOverWeek       = "weekOverWeek"
	PeriodQuarterOverQuarter = "quarterOverQuarter"
	PeriodYearOverYear       = "yearOverYear"
)

// Dimension constants
const (
	DimensionCategory = "category"
	DimensionMerchant = "merchant"
	DimensionOverall  = "overall"
)

// --- Request types ---

type CreateWidgetRequest struct {
	Type          string             `json:"type"`
	Visualization string             `json:"visualization"`
	Config        models.WidgetConfig `json:"config"`
}

type UpdateWidgetConfigRequest struct {
	Config models.WidgetConfig `json:"config"`
}

type ReorderWidgetItem struct {
	WidgetID string `json:"widgetId"`
	Position int    `json:"position"`
}

type ReorderWidgetsRequest struct {
	WidgetOrder []ReorderWidgetItem `json:"widgetOrder"`
}

// --- Widget data response types ---

type WidgetDataResponse struct {
	WidgetID    string    `json:"widgetId"`
	Data        any       `json:"data"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// TopSpendersData is returned for topSpenders widgets (categories or merchants).
type TopSpendersData struct {
	Dimension   string        `json:"dimension"`
	TotalAmount float64       `json:"totalAmount"`
	Currency    string        `json:"currency"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	Items       []SpenderItem `json:"items"`
}

type SpenderItem struct {
	Name    string  `json:"name"`
	Amount  float64 `json:"amount"`
	Percent float64 `json:"percent"`
	Count   int     `json:"count"`
}

// PeriodComparisonWidgetData is returned for periodComparison widgets.
type PeriodComparisonWidgetData struct {
	Current  PeriodDataItem `json:"current"`
	Previous PeriodDataItem `json:"previous"`
	Change   ChangeDataItem `json:"change"`
}

type PeriodDataItem struct {
	Amount   float64 `json:"amount"`
	Count    int     `json:"count"`
	Currency string  `json:"currency"`
	From     string  `json:"from"`
	To       string  `json:"to"`
}

type ChangeDataItem struct {
	Amount  float64  `json:"amount"`
	Percent *float64 `json:"percent"`
	Count   int      `json:"count"`
}

// LargestTransactionsData is returned for largestTransactions widgets.
// Each item includes both merchant and category so the client can display either.
type LargestTransactionsData struct {
	Transactions []TransactionWidgetItem `json:"transactions"`
}

type TransactionWidgetItem struct {
	TransactionID string  `json:"transactionId"`
	Date          string  `json:"date"`
	Merchant      string  `json:"merchant"`
	Amount        float64 `json:"amount"`
	Category      string  `json:"category"`
}

// RecurringSubscriptionsData is returned for recurringSubscriptions widgets.
type RecurringSubscriptionsData struct {
	Subscriptions []SubscriptionItem `json:"subscriptions"`
	TotalMonthly  float64            `json:"totalMonthly"`
	Currency      string             `json:"currency"`
}

type SubscriptionItem struct {
	Merchant     string  `json:"merchant"`
	Amount       float64 `json:"amount"`
	Frequency    string  `json:"frequency"`
	MonthlyEquiv float64 `json:"monthlyEquiv"`
	Variable     bool    `json:"variable"`
}
