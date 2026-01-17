package dto

type TransactionQuery struct {
	Pending    *bool
	PFCPrimary *string
	BankID     *string
	DateFrom   *string
	DateTo     *string
	OrderBy    string
	Desc       bool
	Limit      int
}
