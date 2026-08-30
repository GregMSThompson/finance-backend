package models

import "time"

// OrphanedBank records a Plaid Item we failed to remove during a bank
// deletion. The user's local bank data is deleted regardless (so their delete
// isn't blocked), but the Item is still live at Plaid — still billed and still
// emitting webhooks — so we park the access token and context here to remove it
// later, manually or via automation. Stored in the global orphanedBanks
// collection, keyed by BankID (== Plaid item id) so a retry overwrites rather
// than duplicates.
type OrphanedBank struct {
	BankID      string `firestore:"bankId" json:"bankId"` // also the Plaid item id
	UID         string `firestore:"uid" json:"uid"`
	Institution string `firestore:"institution" json:"institution"`
	// AccessToken is the long-lived Plaid access token needed to remove the Item.
	// KMS-encrypted at rest and never serialized, same as models.Bank.
	AccessToken string    `firestore:"accessToken" json:"-"`
	Error       string    `firestore:"error" json:"error"`
	CreatedAt   time.Time `firestore:"createdAt" json:"createdAt"`
}
