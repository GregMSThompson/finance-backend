package models

import "time"

// NotificationSource identifies which subsystem produced a notification. It is
// data for history filtering and client-side deep-link routing — the delivery
// path never branches on it.
type NotificationSource string

const (
	NotificationSourceAlert NotificationSource = "alert"
	NotificationSourceGoal  NotificationSource = "goal"
)

// Notification is a fully-formed, source-agnostic message block stored under
// users/{uid}/notifications. Both the alert and goal evaluators build one; the
// worker delivers it without needing to know where it came from. It doubles as
// the notification history record and carries its own delivery-idempotency via
// DeliveredAt.
type Notification struct {
	NotificationID string             `firestore:"notificationId" json:"notificationId"`
	Source         NotificationSource `firestore:"source" json:"source"`
	// SourceID is the originating alertId / goalId, for traceability and the
	// client's deep-link target.
	SourceID string `firestore:"sourceId" json:"sourceId"`
	Title    string `firestore:"title" json:"title"`
	Body     string `firestore:"body" json:"body"`
	// Data is the FCM data payload sent alongside the notification, used by the
	// client to route the deep-link when the user taps it.
	Data        map[string]string `firestore:"data,omitempty" json:"data,omitempty"`
	Delivery    DeliveryMethod    `firestore:"delivery" json:"delivery"`
	CreatedAt   time.Time         `firestore:"createdAt" json:"createdAt"`
	DeliveredAt *time.Time        `firestore:"deliveredAt,omitempty" json:"deliveredAt,omitempty"`
}
