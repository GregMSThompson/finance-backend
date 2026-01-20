package dto

type AIQueryRequest struct {
	Message string `json:"message"`
}

type AIQueryResponse struct {
	Answer string       `json:"answer"`
	Debug  *AIDebugInfo `json:"debug,omitempty"`
}

type AIDebugInfo struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}
