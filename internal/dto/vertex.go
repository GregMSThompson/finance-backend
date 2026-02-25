package dto

type VertexGenerateRequest struct {
	Model           string
	System          string
	Contents        []VertexContent // Structured conversation history
	Tools           []VertexTool
	ToolConfig      *VertexToolConfig
	Temperature     *float32
	MaxOutputTokens *int32
}

type VertexContent struct {
	Role  string       // "user" or "model"
	Parts []VertexPart
}

type VertexPart struct {
	Text             *string
	FunctionCall     *VertexToolCall
	FunctionResponse *VertexToolResult
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

type FunctionCallingMode string

const (
	FunctionCallingModeAuto FunctionCallingMode = "AUTO"
	FunctionCallingModeAny  FunctionCallingMode = "ANY"
	FunctionCallingModeNone FunctionCallingMode = "NONE"
)

type VertexToolConfig struct {
	Mode FunctionCallingMode
}
