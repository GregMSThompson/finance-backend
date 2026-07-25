package models

import "time"

// GoalTriggerReason identifies why a goal notification was raised. It forms part
// of the dedup key so the same event isn't sent twice on batch retry.
type GoalTriggerReason string

const (
	GoalTriggerProgressThreshold GoalTriggerReason = "progress_threshold"
	GoalTriggerSingleTransaction GoalTriggerReason = "single_transaction"
	GoalTriggerMidpoint          GoalTriggerReason = "midpoint"
	GoalTriggerProjectedOverage  GoalTriggerReason = "projected_overage"
	GoalTriggerCompletedSuccess  GoalTriggerReason = "completed_success"
	GoalTriggerCompletedFailed   GoalTriggerReason = "completed_failed"
	GoalTriggerRecurringReset    GoalTriggerReason = "recurring_reset"
)

// GoalNotification is a dedup log entry recording that a notification was sent
// for a goal, stored under users/{uid}/goalNotifications. The evaluator checks
// it before sending to avoid duplicate pushes when the batch job retries.
type GoalNotification struct {
	NotificationID string            `firestore:"notificationId" json:"notificationId"`
	GoalID         string            `firestore:"goalId" json:"goalId"`
	TriggerReason  GoalTriggerReason `firestore:"triggerReason" json:"triggerReason"`
	Message        string            `firestore:"message" json:"message"`
	SentAt         time.Time         `firestore:"sentAt" json:"sentAt"`
}
