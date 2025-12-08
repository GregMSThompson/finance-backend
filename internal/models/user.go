package models

import (
	"time"
)

type User struct {
	UID       string    `firestore:"uid" json:"uid"`
	Email     string    `firestore:"email" json:"email"`
	FirstName string    `firestore:"firstName" json:"firstName"`
	LastName  string    `firestore:"lastName" json:"lastName"`
	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `firestore:"updatedAt" json:"updatedAt"`
}
