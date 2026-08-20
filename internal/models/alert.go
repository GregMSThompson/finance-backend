package models

import "time"

// DeliveryMethod is the channel through which an alert notification is sent.
type DeliveryMethod string

const (
	DeliveryEmail DeliveryMethod = "email"
	DeliverySMS   DeliveryMethod = "sms"
	DeliveryPush  DeliveryMethod = "push"
)

// AlertType identifies the kind of condition the alert monitors.
type AlertType string

const (
	AlertTypeSpendThreshold       AlertType = "spend_threshold"
	AlertTypeLargeTransaction     AlertType = "large_transaction"
	AlertTypeSubscriptionIncrease AlertType = "subscription_increase"
	AlertTypeIncomeReceived       AlertType = "income_received"
)

// Alert represents a user's alert configuration stored in Firestore.
type Alert struct {
	AlertID   string           `firestore:"alertId" json:"alertId"`
	Type      AlertType        `firestore:"type" json:"type"`
	Enabled   bool             `firestore:"enabled" json:"enabled"`
	Delivery  []DeliveryMethod `firestore:"delivery" json:"delivery"`
	Config    AlertConfig      `firestore:"config" json:"config"`
	CreatedAt time.Time        `firestore:"createdAt" json:"createdAt"`
	UpdatedAt time.Time        `firestore:"updatedAt" json:"updatedAt"`
}

// AlertConfig holds all possible configuration fields for any alert type.
// Not all fields are valid for all types; the service layer enforces per-type rules.
type AlertConfig struct {
	AmountMinor int64  `firestore:"amountMinor,omitempty" json:"amountMinor,omitempty"`
	Window      string `firestore:"window,omitempty" json:"window,omitempty"`
	Dimension   string `firestore:"dimension,omitempty" json:"dimension,omitempty"` // "overall", "category", "merchant"
	Category    string `firestore:"category,omitempty" json:"category,omitempty"`
	Merchant    string `firestore:"merchant,omitempty" json:"merchant,omitempty"`
}
