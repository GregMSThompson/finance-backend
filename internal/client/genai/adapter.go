package genaiclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/genai"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type Adapter struct {
	client *genai.Client
	model  string
}

func NewAdapter(ctx context.Context, projectID, region, model string) (*Adapter, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: region,
	})
	if err != nil {
		return nil, errs.NewExternalServiceError("genai", "failed to create Gen AI client", false, err)
	}
	return &Adapter{client: client, model: model}, nil
}

func (a *Adapter) GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error) {
	out := dto.VertexGenerateResponse{}

	modelName := req.Model
	if modelName == "" {
		modelName = a.model
	}
	if modelName == "" {
		return out, fmt.Errorf("genai model is required")
	}

	cfg := &genai.GenerateContentConfig{}

	if req.System != "" {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(req.System)},
		}
	}
	if req.Temperature != nil {
		cfg.Temperature = req.Temperature
	}
	if req.MaxOutputTokens != nil {
		cfg.MaxOutputTokens = *req.MaxOutputTokens
	}
	if len(req.Tools) > 0 {
		cfg.Tools = toGenaiTools(req.Tools)
	}
	if req.ToolConfig != nil {
		cfg.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigMode(req.ToolConfig.Mode),
			},
		}
	}

	contents := toGenaiContents(req.Contents)
	if len(contents) == 0 {
		return out, fmt.Errorf("genai generate request has no content")
	}

	if logger.IsDebugEnabled(ctx) {
		log := logger.FromContext(ctx)
		toolSummary := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			var propCount int
			var required []string
			enumSizes := map[string]int{}
			if tool.Parameters != nil {
				required = tool.Parameters.Required
				propCount = len(tool.Parameters.Properties)
				for name, prop := range tool.Parameters.Properties {
					if prop != nil && len(prop.Enum) > 0 {
						enumSizes[name] = len(prop.Enum)
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
		log.Debug("genai generate content request",
			"systemLen", len(req.System),
			"contents", len(contents),
			"tools", toolSummary,
		)
	}

	resp, err := a.client.Models.GenerateContent(ctx, modelName, contents, cfg)
	if err != nil {
		return out, errs.NewExternalServiceError("genai", "failed to generate content", IsTransientError(err), err)
	}

	out.Raw = resp

	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
		return out, errs.NewExternalServiceError("genai", fmt.Sprintf("content blocked: %v", resp.PromptFeedback.BlockReason), false, nil)
	}

	malformed := false
	for _, candidate := range resp.Candidates {
		switch candidate.FinishReason {
		case genai.FinishReasonSafety:
			return out, errs.NewExternalServiceError("genai", "response blocked by safety filters", false, nil)
		case genai.FinishReasonMalformedFunctionCall:
			malformed = true
		}
	}

	out.Text = resp.Text()
	for _, fc := range resp.FunctionCalls() {
		out.ToolCalls = append(out.ToolCalls, dto.VertexToolCall{
			Name: fc.Name,
			Args: fc.Args,
		})
	}

	if logger.IsDebugEnabled(ctx) {
		log := logger.FromContext(ctx)
		finishReasons := make([]string, 0, len(resp.Candidates))
		partsDebug := make([]map[string]any, 0)
		for _, candidate := range resp.Candidates {
			finishReasons = append(finishReasons, string(candidate.FinishReason))
			if candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				switch {
				case part.Text != "":
					partsDebug = append(partsDebug, map[string]any{
						"type":   "text",
						"length": len(part.Text),
					})
				case part.FunctionCall != nil:
					partsDebug = append(partsDebug, map[string]any{
						"type": "functionCall",
						"name": part.FunctionCall.Name,
						"args": part.FunctionCall.Args,
					})
				default:
					partsDebug = append(partsDebug, map[string]any{"type": "other"})
				}
			}
		}
		log.Debug("genai generate content response",
			"candidates", len(resp.Candidates),
			"toolCalls", len(out.ToolCalls),
			"textLen", len(out.Text),
			"finishReasons", finishReasons,
			"parts", partsDebug,
		)
	}

	if len(out.Text) == 0 && len(out.ToolCalls) == 0 {
		if malformed {
			return out, errs.NewMalformedFunctionCallError()
		}
		return out, fmt.Errorf("genai response contained no text or tool calls")
	}

	return out, nil
}

func toGenaiContents(contents []dto.VertexContent) []*genai.Content {
	result := make([]*genai.Content, 0, len(contents))
	for _, c := range contents {
		result = append(result, &genai.Content{
			Role:  c.Role,
			Parts: toGenaiParts(c.Parts),
		})
	}
	return result
}

func toGenaiParts(parts []dto.VertexPart) []*genai.Part {
	result := make([]*genai.Part, 0, len(parts))
	for _, p := range parts {
		if p.Text != nil {
			result = append(result, genai.NewPartFromText(*p.Text))
		}
		if p.FunctionCall != nil {
			result = append(result, genai.NewPartFromFunctionCall(p.FunctionCall.Name, p.FunctionCall.Args))
		}
		if p.FunctionResponse != nil {
			result = append(result, genai.NewPartFromFunctionResponse(p.FunctionResponse.Name, p.FunctionResponse.Response))
		}
	}
	return result
}

func toGenaiTools(tools []dto.VertexTool) []*genai.Tool {
	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  toGenaiSchema(tool.Parameters),
		})
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

func toGenaiSchema(schema *dto.VertexSchema) *genai.Schema {
	if schema == nil {
		return nil
	}
	out := &genai.Schema{
		Type:        genai.Type(schema.Type),
		Description: schema.Description,
		Enum:        schema.Enum,
		Required:    schema.Required,
	}
	if schema.Items != nil {
		out.Items = toGenaiSchema(schema.Items)
	}
	if len(schema.Properties) > 0 {
		out.Properties = make(map[string]*genai.Schema, len(schema.Properties))
		for k, v := range schema.Properties {
			out.Properties[k] = toGenaiSchema(v)
		}
	}
	return out
}

// IsTransientError reports whether a Gen AI API error is worth retrying.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *genai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusTooManyRequests,    // 429
			http.StatusInternalServerError, // 500
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
		return false
	}
	return false
}
