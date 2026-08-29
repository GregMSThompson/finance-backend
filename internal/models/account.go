package models

import "time"

// Account represents a Plaid account (checking, savings, credit card, etc.)
// linked to a bank Item. Multiple accounts may share a parent bank when a user
// authenticates a single institution login that exposes more than one product.
// Stored at users/{uid}/banks/{bankId}/accounts/{accountId}.
type Account struct {
	AccountID             string    `firestore:"accountId" json:"accountId"`
	Name                  string    `firestore:"name" json:"name"`
	OfficialName          string    `firestore:"officialName" json:"officialName,omitempty"`
	Type                  string    `firestore:"type" json:"type"`
	Subtype               string    `firestore:"subtype" json:"subtype,omitempty"`
	Mask                  string    `firestore:"mask" json:"mask,omitempty"`
	BalanceAvailableMinor *int64    `firestore:"balanceAvailableMinor" json:"balanceAvailableMinor"`
	BalanceCurrentMinor   *int64    `firestore:"balanceCurrentMinor" json:"balanceCurrentMinor"`
	BalanceLimitMinor     *int64    `firestore:"balanceLimitMinor" json:"balanceLimitMinor"`
	Currency              string    `firestore:"currency" json:"currency"`
	CreatedAt             time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt             time.Time `firestore:"updatedAt" json:"updatedAt"`
}
