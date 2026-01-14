package models

import (
	"time"
)

type Transaction struct {
	TransactionID  string    `firestore:"transactionId" json:"transactionId"` // Plaid transaction_id (doc ID)
	BankID         string    `firestore:"bankId" json:"bankId"`               // Plaid item_id
	Name           string    `firestore:"name" json:"name"`
	Amount         float64   `firestore:"amount" json:"amount"`
	Currency       string    `firestore:"currency" json:"currency"`
	Pending        bool      `firestore:"pending" json:"pending"`
	Date           string    `firestore:"date" json:"date"` // YYYY-MM-DD as Plaid returns
	AuthorizedDate string    `firestore:"authorizedDate" json:"authorizedDate,omitempty"`
	Categories     []string  `firestore:"categories" json:"categories,omitempty"`
	PFCPrimary     string    `firestore:"pfcPrimary" json:"pfcPrimary,omitempty"`
	PFCDetailed    string    `firestore:"pfcDetailed" json:"pfcDetailed,omitempty"`
	PFCConfidence  string    `firestore:"pfcConfidence" json:"pfcConfidence,omitempty"`
	PFCIconURL     string    `firestore:"pfcIconUrl" json:"pfcIconUrl,omitempty"`
	CreatedAt      time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time `firestore:"updatedAt" json:"updatedAt"`
}
