package vertexclient

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type Adapter struct {
	client *genai.Client
	model  string
	log    *slog.Logger
}

func NewAdapter(ctx context.Context, log *slog.Logger, projectID, region, model string) (*Adapter, error) {
	client, err := genai.NewClient(ctx, projectID, region)
	if err != nil {
		return nil, errs.NewExternalServiceError("vertex", "failed to create Vertex AI client", IsTransientError(err), err)
	}

	return &Adapter{
		client: client,
		model:  model,
		log:    log,
	}, nil
}

func (a *Adapter) Close() error {
	err := a.client.Close()
	if err != nil && a.log != nil {
		a.log.Error("vertex adapter close failed", "error", err)
	}
	return err
}

func (a *Adapter) GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error) {
	out := dto.VertexGenerateResponse{}

	modelName := req.Model
	if modelName == "" {
		modelName = a.model
	}
	if modelName == "" {
		return out, fmt.Errorf("vertex model is required")
	}

	model := a.client.GenerativeModel(modelName)
	if req.System != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(req.System)},
		}
	}
	if req.Temperature != nil {
		model.SetTemperature(*req.Temperature)
	}
	if req.MaxOutputTokens != nil {
		model.SetMaxOutputTokens(*req.MaxOutputTokens)
	}
	if len(req.Tools) > 0 {
		model.Tools = toGenaiTools(req.Tools)
	}
	if req.ToolConfig != nil {
		model.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: toGenaiMode(req.ToolConfig.Mode),
			},
		}
	}

	// Only build tool summaries for debug logs when debug is enabled.
	if logger.IsDebugEnabled(ctx) {
		log := logger.FromContext(ctx)
		toolSummary := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			propCount := 0
			enumSizes := map[string]int{}
			required := []string(nil)
			if tool.Parameters != nil {
				required = tool.Parameters.Required
				if len(tool.Parameters.Properties) > 0 {
					propCount = len(tool.Parameters.Properties)
					for name, prop := range tool.Parameters.Properties {
						if prop != nil && len(prop.Enum) > 0 {
							enumSizes[name] = len(prop.Enum)
						}
					}
				}
			}
			toolSummary = append(toolSummary, map[string]any{
				"name":       tool.Name,
				"required":   required,
				"properties": propCount,
				"enumSizes":  enumSizes,
			})
		}
		log.Debug(
			"vertex generate content request",
			"systemLen", len(req.System),
			"contents", len(req.Contents),
			"tools", toolSummary,
		)
	}

	if len(req.Contents) == 0 {
		return out, fmt.Errorf("vertex generate request has no content")
	}

	// Split contents into history and current message
	var history []*genai.Content
	var currentParts []genai.Part

	if len(req.Contents) > 1 {
		// Convert all but last to history
		history = toGenaiContents(req.Contents[:len(req.Contents)-1])
	}

	// Last content is the current message
	currentParts = toGenaiParts(req.Contents[len(req.Contents)-1].Parts)

	// Use chat session with history
	chat := model.StartChat()
	chat.History = history

	resp, err := chat.SendMessage(ctx, currentParts...)
	if err != nil {
		return out, errs.NewExternalServiceError("vertex", "failed to generate content", IsTransientError(err), err)
	}

	out.Raw = resp

	// Check for blocked content due to safety filters
	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != 0 {
		return out, errs.NewExternalServiceError("vertex", fmt.Sprintf("content blocked: %v", resp.PromptFeedback.BlockReason), false, nil)
	}

	out.Text, out.ToolCalls = parseContentResponse(resp)

	// Check for malformed function calls and empty responses
	malformed := false
	blocked := false
	for _, candidate := range resp.Candidates {
		if candidate.FinishReason == genai.FinishReasonMalformedFunctionCall {
			malformed = true
			break
		}
		if candidate.FinishReason == genai.FinishReasonSafety {
			blocked = true
			break
		}
	}

	if blocked {
		return out, errs.NewExternalServiceError("vertex", "response blocked by safety filters", false, nil)
	}

	// Only build response-part summaries for debug logs when debug is enabled.
	if logger.IsDebugEnabled(ctx) {
		log := logger.FromContext(ctx)
		finishReasons := make([]string, 0, len(resp.Candidates))
		partsDebug := make([]map[string]any, 0)
		for _, candidate := range resp.Candidates {
			finishReasons = append(finishReasons, candidate.FinishReason.String())
			if candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				switch p := part.(type) {
				case genai.Text:
					partsDebug = append(partsDebug, map[string]any{
						"type":   "text",
						"length": len(p),
					})
				case *genai.Text:
					partsDebug = append(partsDebug, map[string]any{
						"type":   "text",
						"length": len(*p),
					})
				case *genai.FunctionCall:
					partsDebug = append(partsDebug, map[string]any{
						"type": "functionCall",
						"name": p.Name,
						"args": p.Args,
					})
				case genai.FunctionCall:
					partsDebug = append(partsDebug, map[string]any{
						"type": "functionCall",
						"name": p.Name,
						"args": p.Args,
					})
				default:
					partsDebug = append(partsDebug, map[string]any{
						"type": fmt.Sprintf("%T", part),
					})
				}
			}
		}
		log.Debug(
			"vertex generate content response",
			"candidates", len(resp.Candidates),
			"toolCalls", len(out.ToolCalls),
			"textLen", len(out.Text),
			"promptFeedback", resp.PromptFeedback,
			"promptFeedbackRaw", fmt.Sprintf("%+v", resp.PromptFeedback),
			"finishReasons", finishReasons,
			"parts", partsDebug,
		)
	}

	if len(out.Text) == 0 && len(out.ToolCalls) == 0 {
		if malformed {
			return out, errs.NewMalformedFunctionCallError()
		}
		return out, fmt.Errorf("vertex response contained no text or tool calls")
	}
	return out, nil
}

func parseContentResponse(resp *genai.GenerateContentResponse) (string, []dto.VertexToolCall) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", nil
	}

	var text string
	var calls []dto.VertexToolCall
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			switch p := part.(type) {
			case genai.Text:
				text += string(p)
			case *genai.Text:
				text += string(*p)
			case *genai.FunctionCall:
				calls = append(calls, dto.VertexToolCall{
					Name: p.Name,
					Args: p.Args,
				})
			case genai.FunctionCall:
				calls = append(calls, dto.VertexToolCall{
					Name: p.Name,
					Args: p.Args,
				})
			}
		}
	}

	return text, calls
}

func toGenaiContents(contents []dto.VertexContent) []*genai.Content {
	if len(contents) == 0 {
		return nil
	}

	result := make([]*genai.Content, 0, len(contents))
	for _, content := range contents {
		result = append(result, &genai.Content{
			Role:  content.Role,
			Parts: toGenaiParts(content.Parts),
		})
	}
	return result
}

func toGenaiParts(parts []dto.VertexPart) []genai.Part {
	if len(parts) == 0 {
		return nil
	}

	result := make([]genai.Part, 0, len(parts))
	for _, part := range parts {
		if part.Text != nil {
			result = append(result, genai.Text(*part.Text))
		}
		if part.FunctionCall != nil {
			result = append(result, genai.FunctionCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
		if part.FunctionResponse != nil {
			result = append(result, genai.FunctionResponse{
				Name:     part.FunctionResponse.Name,
				Response: part.FunctionResponse.Response,
			})
		}
	}
	return result
}

func toGenaiTools(tools []dto.VertexTool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  toGenaiSchema(tool.Parameters),
		})
	}

	return []*genai.Tool{
		{FunctionDeclarations: decls},
	}
}

func toGenaiSchema(schema *dto.VertexSchema) *genai.Schema {
	if schema == nil {
		return nil
	}

	out := &genai.Schema{
		Type:        toGenaiType(schema.Type),
		Description: schema.Description,
		Enum:        schema.Enum,
		Required:    schema.Required,
	}

	if schema.Items != nil {
		out.Items = toGenaiSchema(schema.Items)
	}
	if len(schema.Properties) > 0 {
		out.Properties = make(map[string]*genai.Schema, len(schema.Properties))
		for key, value := range schema.Properties {
			out.Properties[key] = toGenaiSchema(value)
		}
	}

	return out
}

func toGenaiType(schemaType string) genai.Type {
	switch schemaType {
	case "object":
		return genai.TypeObject
	case "array":
		return genai.TypeArray
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	default:
		return genai.TypeUnspecified
	}
}

func toGenaiMode(mode dto.FunctionCallingMode) genai.FunctionCallingMode {
	switch mode {
	case dto.FunctionCallingModeAuto:
		return genai.FunctionCallingAuto
	case dto.FunctionCallingModeAny:
		return genai.FunctionCallingAny
	case dto.FunctionCallingModeNone:
		return genai.FunctionCallingNone
	default:
		return genai.FunctionCallingAuto
	}
}

// IsTransientError checks if a Vertex AI error is transient (retryable).
// Transient errors include service unavailability, timeouts, and resource exhaustion.
// Non-transient errors include invalid arguments, permission denied, etc.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Extract gRPC status code from error
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, assume non-transient
		return false
	}

	// Check if the error code indicates a transient condition
	switch st.Code() {
	case codes.Unavailable,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Aborted:
		return true

	case codes.InvalidArgument,
		codes.NotFound,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Unimplemented:
		return false
	}

	// For unknown codes, assume non-transient to avoid infinite retries
	return false
}
