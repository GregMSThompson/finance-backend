package models

import (
	"time"
)

type Bank struct {
	BankID      string    `firestore:"bankId" json:"bankId"`
	Institution string    `firestore:"institution" json:"institution"`
	Status      string    `firestore:"status" json:"status"` // e.g. "active", "inactive"
	CreatedAt   time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time `firestore:"updatedAt" json:"updatedAt"`
}
