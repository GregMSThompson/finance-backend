package models

import "time"

// GoalSnapshot is a point-in-time progress record written by the daily
// evaluator (one per goal per run), stored under users/{uid}/goalSnapshots.
// It drives the progress graph and carries the AI insight shown both in-app
// and in the push notification. GoalID is a filter key (snapshots live in a
// per-user collection, not nested under the goal).
type GoalSnapshot struct {
	SnapshotID      string    `firestore:"snapshotId" json:"snapshotId"`
	GoalID          string    `firestore:"goalId" json:"goalId"`
	EvaluatedAt     time.Time `firestore:"evaluatedAt" json:"evaluatedAt"`
	CurrentValue    float64   `firestore:"currentValue" json:"currentValue"`
	TargetValue     float64   `firestore:"targetValue" json:"targetValue"`
	PercentComplete float64   `firestore:"percentComplete" json:"percentComplete"`
	IsOnTrack       bool      `firestore:"isOnTrack" json:"isOnTrack"`
	// AIInsight is the batch-generated 1-2 sentence insight, set only when a
	// notification-worthy event occurred. Same text used for the push body.
	AIInsight        string `firestore:"aiInsight,omitempty" json:"aiInsight,omitempty"`
	NotificationSent bool   `firestore:"notificationSent" json:"notificationSent"`
}
