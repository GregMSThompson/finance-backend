package dto

type VertexGenerateRequest struct {
	Model           string
	System          string
	UserMessage     string
	Tools           []VertexTool
	ToolResults     []VertexToolResult
	Temperature     *float32
	MaxOutputTokens *int32
}

type VertexGenerateResponse struct {
	Text      string
	ToolCalls []VertexToolCall
	Raw       any
}

type VertexTool struct {
	Name        string
	Description string
	Parameters  *VertexSchema
}

type VertexToolCall struct {
	Name string
	Args map[string]any
}

type VertexToolResult struct {
	Name     string
	Response map[string]any
}

type VertexSchema struct {
	Type        string
	Description string
	Enum        []string
	Properties  map[string]*VertexSchema
	Required    []string
	Items       *VertexSchema
}
