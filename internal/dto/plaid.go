package dto

import (
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type LinkBankRequest struct {
	PublicToken     string `json:"publicToken"`
	InstitutionName string `json:"institutionName,omitempty"`
}

type SyncTransactionsRequest struct {
	BankID *string `json:"bankId,omitempty"`
}

// PlaidSyncParams is the params payload for a JobTypePlaidSync job.
// Stored as the Job.Params JSON blob.
type PlaidSyncParams struct {
	BankID *string `json:"bankId,omitempty"`
}

// PlaidWebhook is the inbound payload Plaid POSTs to /plaid/webhook.
// Only the fields the service branches on are decoded; everything else is dropped.
type PlaidWebhook struct {
	WebhookType string             `json:"webhook_type"`
	WebhookCode string             `json:"webhook_code"`
	ItemID      string             `json:"item_id"`
	Environment string             `json:"environment,omitempty"`
	Error       *PlaidWebhookError `json:"error,omitempty"`
}

// PlaidWebhookError is the nested error block on ITEM:ERROR webhooks.
type PlaidWebhookError struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
}

// Metadata from the transaction sync process
type PlaidServiceSyncResult struct {
	BanksSynced          int
	TransactionsInserted int
	TransactionsUpdated  int
	Cursor               string // latest cursor if syncing one bank; empty when multiple
}

// Paid adapter result - represents one page from /transactions/sync
type PlaidSyncPage struct {
	Transactions []models.Transaction
	Cursor       string
	HasMore      bool
}

type PlaidEnvironment string

const (
	PlaidSandbox     PlaidEnvironment = "sandbox"
	PalidDevelopment PlaidEnvironment = "development"
	PlaidProduction  PlaidEnvironment = "production"
)
