package models

import "time"

// AlertEvent is a record of a single triggered alert delivery.
// One event is created per delivery channel per trigger — e.g. an alert with
// delivery ["push", "email"] produces two events when it fires.
type AlertEvent struct {
	EventID  string         `firestore:"eventId" json:"eventId"`
	AlertID  string         `firestore:"alertId" json:"alertId"`
	UserID   string         `firestore:"userId" json:"userId"`
	Type     AlertType      `firestore:"type" json:"type"`
	Delivery DeliveryMethod `firestore:"delivery" json:"delivery"`
	// Title and Body are pre-built by the evaluator so the worker
	// can deliver without re-deriving message content.
	Title       string     `firestore:"title" json:"title"`
	Body        string     `firestore:"body" json:"body"`
	TriggeredAt time.Time  `firestore:"triggeredAt" json:"triggeredAt"`
	DeliveredAt *time.Time `firestore:"deliveredAt,omitempty" json:"deliveredAt,omitempty"`
}
