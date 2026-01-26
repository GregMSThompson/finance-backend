package vertexclient

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/vertexai/genai"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type Adapter struct {
	client *genai.Client
	model  string
	log    *slog.Logger
}

func NewAdapter(ctx context.Context, log *slog.Logger, projectID, region, model string) (*Adapter, error) {
	client, err := genai.NewClient(ctx, projectID, region)
	if err != nil {
		return nil, err
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

	// debug request metadata
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
	a.log.Debug(
		"vertex generate content request",
		"systemLen", len(req.System),
		"userLen", len(req.UserMessage),
		"tools", toolSummary,
		"toolResults", len(req.ToolResults),
	)

	var parts []genai.Part
	if req.UserMessage != "" {
		parts = append(parts, genai.Text(req.UserMessage))
	}
	for _, toolResult := range req.ToolResults {
		parts = append(parts, genai.FunctionResponse{
			Name:     toolResult.Name,
			Response: toolResult.Response,
		})
	}
	if len(parts) == 0 {
		return out, fmt.Errorf("vertex generate request has no content")
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return out, err
	}

	out.Raw = resp
	out.Text, out.ToolCalls = parseContentResponse(resp)

	// debug gemini output
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
	a.log.Debug(
		"vertex generate content response",
		"candidates", len(resp.Candidates),
		"toolCalls", len(out.ToolCalls),
		"textLen", len(out.Text),
		"promptFeedback", resp.PromptFeedback,
		"finishReasons", finishReasons,
		"parts", partsDebug,
	)

	if len(out.Text) == 0 && len(out.ToolCalls) == 0 {
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
