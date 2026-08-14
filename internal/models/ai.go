package models

import "time"

type AIMessage struct {
	Role       string         `firestore:"role" json:"role"`
	Content    string         `firestore:"content,omitempty" json:"content,omitempty"`
	ToolName   string         `firestore:"toolName,omitempty" json:"toolName,omitempty"`
	ToolArgs   map[string]any `firestore:"toolArgs,omitempty" json:"toolArgs,omitempty"`
	ToolResult map[string]any `firestore:"toolResult,omitempty" json:"toolResult,omitempty"`
	CreatedAt  time.Time      `firestore:"createdAt" json:"createdAt"`
}

// AISession is the metadata document for a chat session, stored at
// users/{uid}/ai_sessions/{sessionId} alongside its messages subcollection. It
// exists so conversations can be listed without reading their messages — Title
// is a snippet of the first user message, CreatedAt/UpdatedAt drive recency.
type AISession struct {
	SessionID string    `firestore:"sessionId" json:"sessionId"`
	Title     string    `firestore:"title" json:"title"`
	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `firestore:"updatedAt" json:"updatedAt"`
}
