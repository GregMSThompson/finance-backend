package dto

import "time"

type AIMessage struct {
	Role       string         `firestore:"role" json:"role"`
	Content    string         `firestore:"content,omitempty" json:"content,omitempty"`
	ToolName   string         `firestore:"toolName,omitempty" json:"toolName,omitempty"`
	ToolArgs   map[string]any `firestore:"toolArgs,omitempty" json:"toolArgs,omitempty"`
	ToolResult map[string]any `firestore:"toolResult,omitempty" json:"toolResult,omitempty"`
	CreatedAt  time.Time      `firestore:"createdAt" json:"createdAt"`
	ExpiresAt  time.Time      `firestore:"expiresAt,omitempty" json:"expiresAt,omitempty"`
}
