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
