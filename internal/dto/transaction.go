package dto

import "github.com/GregMSThompson/finance-backend/internal/models"

type TransactionQuery struct {
	Pending      *bool
	PFCPrimaries []string
	AccountID    *string
	Merchant     *string
	DateFrom     *string
	DateTo       *string
	OrderBy      string
	Desc         bool
	Limit        int
	// Cursor is an opaque value pointing at a position in the sorted stream;
	// when set, the store skips rows up to and including the cursor row.
	Cursor *string
}

type TransactionListArgs struct {
	Pending      *bool
	PFCPrimaries []string
	AccountID    *string
	Merchant     *string
	DateFrom     *string
	DateTo       *string
	OrderBy      string
	Desc         bool
	Limit        int
	Cursor       *string
}

type TransactionListResult struct {
	Transactions []models.Transaction `json:"transactions"`
	// NextCursor, if present, is an opaque token the client should pass back
	// as ?cursor= to fetch the next page using the same filters and sort.
	NextCursor *string `json:"nextCursor,omitempty"`
}
