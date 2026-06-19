package dto

import "github.com/GregMSThompson/finance-backend/internal/models"

type TransactionQuery struct {
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

type TransactionListArgs struct {
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

type TransactionListResult struct {
	Transactions []models.Transaction `json:"transactions"`
}
