package dto

import "github.com/GregMSThompson/finance-backend/internal/models"

type AnalyticsSpendTotalArgs struct {
	Pending    *bool
	PFCPrimary *string
	BankID     *string
	Merchant   *string
	DateFrom   *string
	DateTo     *string
}

type AnalyticsSpendTotalResult struct {
	Total    float64 `json:"total"`
	Currency string  `json:"currency"`
	From     string  `json:"from,omitempty"`
	To       string  `json:"to,omitempty"`
}

type AnalyticsSpendBreakdownArgs struct {
	Pending    *bool
	PFCPrimary *string
	BankID     *string
	DateFrom   *string
	DateTo     *string
	GroupBy    string
}

type AnalyticsBreakdownItem struct {
	Key   string  `json:"key"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

type AnalyticsSpendBreakdownResult struct {
	GroupBy  string                   `json:"groupBy"`
	Items    []AnalyticsBreakdownItem `json:"items"`
	Currency string                   `json:"currency"`
	From     string                   `json:"from,omitempty"`
	To       string                   `json:"to,omitempty"`
}

type AnalyticsTransactionsArgs struct {
	Pending    *bool
	PFCPrimary *string
	BankID     *string
	Merchant   *string
	DateFrom   *string
	DateTo     *string
	OrderBy    string
	Desc       bool
	Limit      int
}

type AnalyticsTransactionsResult struct {
	Transactions []models.Transaction `json:"transactions"`
}

type AnalyticsPeriodComparisonArgs struct {
	Pending      *bool
	PFCPrimary   *string
	BankID       *string
	Merchant     *string
	CurrentFrom  string
	CurrentTo    string
	PreviousFrom string
	PreviousTo   string
	GroupBy      string
}

type PeriodSummary struct {
	Total    float64                  `json:"total"`
	Count    int                      `json:"count"`
	Currency string                   `json:"currency"`
	From     string                   `json:"from"`
	To       string                   `json:"to"`
	Items    []AnalyticsBreakdownItem `json:"items,omitempty"`
}

type BreakdownItemChange struct {
	Key              string   `json:"key"`
	AbsoluteChange   float64  `json:"absoluteChange"`
	PercentageChange *float64 `json:"percentageChange,omitempty"`
	CountChange      int      `json:"countChange"`
}

type PeriodChange struct {
	AbsoluteChange   float64              `json:"absoluteChange"`
	PercentageChange *float64             `json:"percentageChange,omitempty"`
	CountChange      int                  `json:"countChange"`
	Items            []BreakdownItemChange `json:"items,omitempty"`
}

type AnalyticsPeriodComparisonResult struct {
	GroupBy  string        `json:"groupBy,omitempty"`
	Current  PeriodSummary `json:"current"`
	Previous PeriodSummary `json:"previous"`
	Change   PeriodChange  `json:"change"`
}

type AnalyticsRecurringArgs struct {
	BankID   *string
	DateFrom string
	DateTo   string
}

type RecurringItem struct {
	Merchant          string  `json:"merchant"`
	Frequency         string  `json:"frequency"`
	TypicalAmount     float64 `json:"typicalAmount"`
	AmountIsVariable  bool    `json:"amountIsVariable"`
	Currency          string  `json:"currency"`
	OccurrenceCount   int     `json:"occurrenceCount"`
	LastDate          string  `json:"lastDate"`
	MonthlyEquivalent float64 `json:"monthlyEquivalent"`
}

type RecurringTransactionsResult struct {
	Items                  []RecurringItem `json:"items"`
	TotalMonthlyEquivalent float64         `json:"totalMonthlyEquivalent"`
	Currency               string          `json:"currency"`
	From                   string          `json:"from"`
	To                     string          `json:"to"`
}

type AnalyticsMovingAverageArgs struct {
	Granularity string
	Scope       string
	PFCPrimary  *string
	Merchant    *string
	BankID      *string
	DateFrom    string
	DateTo      string
}

type MovingAverageDataPoint struct {
	Period           string  `json:"period"`
	Total            float64 `json:"total"`
	TransactionCount int     `json:"transactionCount"`
}

type MovingAverageItem struct {
	Key              string                   `json:"key"`
	AveragePerUnit   float64                  `json:"averagePerUnit"`
	TransactionCount int                      `json:"transactionCount"`
	Series           []MovingAverageDataPoint `json:"series"`
}

type AnalyticsTopNArgs struct {
	Dimension  string
	Direction  string // "top" or "bottom"
	Limit      int
	MinCount   int
	PFCPrimary *string
	BankID     *string
	DateFrom   string
	DateTo     string
}

type TopNItem struct {
	Key        string  `json:"key"`
	Total      float64 `json:"total"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type AnalyticsTopNResult struct {
	Dimension  string     `json:"dimension"`
	Direction  string     `json:"direction"`
	TotalSpend float64    `json:"totalSpend"`
	Currency   string     `json:"currency"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Items      []TopNItem `json:"items"`
}

type AnalyticsMovingAverageResult struct {
	Granularity      string                   `json:"granularity"`
	Scope            string                   `json:"scope"`
	AveragePerUnit   float64                  `json:"averagePerUnit"`
	TransactionCount int                      `json:"transactionCount"`
	DaysAnalyzed     int                      `json:"daysAnalyzed"`
	Currency         string                   `json:"currency"`
	From             string                   `json:"from"`
	To               string                   `json:"to"`
	Series           []MovingAverageDataPoint `json:"series,omitempty"`
	Items            []MovingAverageItem      `json:"items,omitempty"`
}
