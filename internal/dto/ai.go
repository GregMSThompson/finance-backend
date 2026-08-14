package dto

import "time"

type AIQueryRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// AIConversationResponse is a stored chat session's messages in chronological
// order, for replaying a previous conversation.
type AIConversationResponse struct {
	SessionID string          `json:"sessionId"`
	Messages  []AIMessageView `json:"messages"`
}

// AIMessageView is a single conversation turn — the client-facing shape, without
// the model's internal tool-call bookkeeping.
type AIMessageView struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type AIQueryResponse struct {
	Answer string       `json:"answer"`
	Debug  *AIDebugInfo `json:"debug,omitempty"`
}

type AIDebugInfo struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}
