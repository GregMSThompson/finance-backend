package models

import (
	"encoding/json"
	"time"
)

// JobType identifies the kind of work an async job performs.
// The string value maps to a worker route by replacing "." with "/".
// e.g. "plaid.sync" → POST /tasks/plaid/sync
type JobType string

const (
	JobTypePlaidSync   JobType = "plaid.sync"
	JobTypeBankDelete  JobType = "bank.delete"
	JobTypeAccountSync JobType = "account.sync"
)

func (t JobType) IsValid() bool {
	switch t {
	case JobTypePlaidSync, JobTypeBankDelete, JobTypeAccountSync:
		return true
	}
	return false
}

// JobStatus tracks the lifecycle state of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job represents an async unit of work tracked in Firestore.
// Params and Result are opaque JSON payloads whose shape is defined per JobType
// in the dto package.
type Job struct {
	JobID       string          `firestore:"jobId" json:"jobId"`
	Type        JobType         `firestore:"type" json:"type"`
	Status      JobStatus       `firestore:"status" json:"status"`
	Params      json.RawMessage `firestore:"params" json:"params,omitempty"`
	Result      json.RawMessage `firestore:"result,omitempty" json:"result,omitempty"`
	Error       string          `firestore:"error,omitempty" json:"error,omitempty"`
	CreatedAt   time.Time       `firestore:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time       `firestore:"updatedAt" json:"updatedAt"`
	CompletedAt *time.Time      `firestore:"completedAt,omitempty" json:"completedAt,omitempty"`
}