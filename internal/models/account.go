package models

import "time"

// Account represents a Plaid account (checking, savings, credit card, etc.)
// linked to a bank Item. Multiple accounts may share a parent bank when a user
// authenticates a single institution login that exposes more than one product.
// Stored at users/{uid}/banks/{bankId}/accounts/{accountId}.
type Account struct {
	AccountID    string    `firestore:"accountId" json:"accountId"`
	Name         string    `firestore:"name" json:"name"`
	OfficialName string    `firestore:"officialName" json:"officialName,omitempty"`
	Type         string    `firestore:"type" json:"type"`
	Subtype      string    `firestore:"subtype" json:"subtype,omitempty"`
	Mask         string    `firestore:"mask" json:"mask,omitempty"`
	CreatedAt    time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time `firestore:"updatedAt" json:"updatedAt"`
}
