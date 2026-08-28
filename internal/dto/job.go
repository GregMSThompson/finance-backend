package dto

// JobTaskRequest is the body Cloud Tasks delivers to every worker task handler.
// Handlers re-load the full job from the store using these identifiers.
type JobTaskRequest struct {
	UID   string `json:"uid"`
	JobID string `json:"jobId"`
}

// SubmitJobResponse is returned by API endpoints that submit async work.
type SubmitJobResponse struct {
	JobID string `json:"jobId"`
}
