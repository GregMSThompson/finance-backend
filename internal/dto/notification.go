package dto

import "github.com/GregMSThompson/finance-backend/internal/models"

// DeliverNotificationRequest is the Cloud Tasks payload for delivering a
// notification through a single channel. Source-agnostic — the notification
// itself carries the message content.
type DeliverNotificationRequest struct {
	NotificationID string                `json:"notificationId"`
	UserID         string                `json:"userId"`
	Delivery       models.DeliveryMethod `json:"delivery"`
}
