package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type vertexClient interface {
	GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error)
}

type analyticsClient interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
	GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error)
	GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error)
}

type aiStore interface {
	SaveMessage(ctx context.Context, uid, sessionID string, msg models.AIMessage) error
	ListMessages(ctx context.Context, uid, sessionID string, limit int) ([]models.AIMessage, error)
}

type aiService struct {
	vertex   vertexClient
	analysis analyticsClient
	store    aiStore
	ttl      time.Duration
	clockNow func() time.Time
}

func NewAIService(vertex vertexClient, analysis analyticsClient, store aiStore, ttl time.Duration) *aiService {
	return &aiService{
		vertex:   vertex,
		analysis: analysis,
		store:    store,
		ttl:      ttl,
		clockNow: time.Now,
	}
}

func (s *aiService) Query(ctx context.Context, uid, sessionID, message string) (dto.AIQueryResponse, error) {
	history, err := s.store.ListMessages(ctx, uid, sessionID, 8)
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	userMsg := s.composeUserMessage(history, message)
	req := dto.VertexGenerateRequest{
		System:      systemPrompt(s.clockNow()),
		UserMessage: userMsg,
		Tools:       toolSchemas(),
	}

	resp, err := s.vertex.GenerateContent(ctx, req)
	if err != nil {
		var malformed *errs.MalformedFunctionCallError
		if errors.As(err, &malformed) {
			strictReq := req
			strictReq.System = strictSystemPrompt(s.clockNow())
			resp, err = s.vertex.GenerateContent(ctx, strictReq)
		}
	}
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	if len(resp.ToolCalls) == 0 {
		if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
			Role:    "user",
			Content: message,
		}); err != nil {
			return dto.AIQueryResponse{}, err
		}
		if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
			Role:    "assistant",
			Content: resp.Text,
		}); err != nil {
			return dto.AIQueryResponse{}, err
		}
		return dto.AIQueryResponse{Answer: resp.Text}, nil
	}

	toolCall := resp.ToolCalls[0]
	toolResult, err := s.executeTool(ctx, uid, toolCall)
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
		Role:    "user",
		Content: message,
	}); err != nil {
		return dto.AIQueryResponse{}, err
	}
	if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
		Role:       "tool",
		ToolName:   toolCall.Name,
		ToolArgs:   toolCall.Args,
		ToolResult: toolResult.Response,
	}); err != nil {
		return dto.AIQueryResponse{}, err
	}

	finalResp, err := s.vertex.GenerateContent(ctx, dto.VertexGenerateRequest{
		System:      systemPrompt(s.clockNow()),
		UserMessage: userMsg,
		ToolResults: []dto.VertexToolResult{toolResult},
	})
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
		Role:    "assistant",
		Content: finalResp.Text,
	}); err != nil {
		return dto.AIQueryResponse{}, err
	}

	return dto.AIQueryResponse{
		Answer: finalResp.Text,
		Debug: &dto.AIDebugInfo{
			Tool: toolCall.Name,
			Args: toolCall.Args,
		},
	}, nil
}

func (s *aiService) composeUserMessage(history []models.AIMessage, message string) string {
	if len(history) == 0 {
		return message
	}

	var b strings.Builder
	for _, msg := range history {
		switch msg.Role {
		case "user":
			b.WriteString("User: ")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		case "assistant":
			b.WriteString("Assistant: ")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		case "tool":
			b.WriteString("Tool ")
			b.WriteString(msg.ToolName)
			b.WriteString(": ")
			if msg.ToolResult != nil {
				if raw, err := json.Marshal(msg.ToolResult); err == nil {
					b.Write(raw)
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("User: ")
	b.WriteString(message)
	return b.String()
}

func (s *aiService) saveMessage(ctx context.Context, uid, sessionID string, msg models.AIMessage) error {
	now := s.clockNow()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	if s.ttl > 0 {
		msg.ExpiresAt = now.Add(s.ttl)
	}
	return s.store.SaveMessage(ctx, uid, sessionID, msg)
}

func (s *aiService) executeTool(ctx context.Context, uid string, call dto.VertexToolCall) (dto.VertexToolResult, error) {
	switch call.Name {
	case "get_spend_total":
		args, err := decodeArgs[dto.AnalyticsSpendTotalArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := s.applyDefaults(&args.Pending, &args.DateFrom, &args.DateTo); err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := validatePrimary(args.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		result, err := s.analysis.GetSpendTotal(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	case "get_spend_breakdown":
		args, err := decodeArgs[dto.AnalyticsSpendBreakdownArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := s.applyDefaults(&args.Pending, &args.DateFrom, &args.DateTo); err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := validatePrimary(args.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		result, err := s.analysis.GetSpendBreakdown(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	case "get_transactions":
		args, err := decodeArgs[dto.AnalyticsTransactionsArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := s.applyDefaults(&args.Pending, &args.DateFrom, &args.DateTo); err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.Limit == 0 {
			args.Limit = 25
		}
		if err := validatePrimary(args.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		result, err := s.analysis.GetTransactions(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	default:
		return dto.VertexToolResult{}, errs.NewValidationError(fmt.Sprintf("unsupported tool: %s", call.Name))
	}
}

func toolSchemas() []dto.VertexTool {
	return []dto.VertexTool{
		{
			Name:        "get_spend_total",
			Description: "Sum transaction amounts with optional filters.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Primary category filter."},
					"pending":    {Type: "boolean", Description: "Defaults to false if omitted."},
					"bankId":     {Type: "string", Description: "Filter by bank id."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start date; defaults to month-to-date."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end date; defaults to today when month-to-date."},
				},
			},
		},
		{
			Name:        "get_spend_breakdown",
			Description: "Group spending totals by category, merchant, or day.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Primary category filter."},
					"pending":    {Type: "boolean", Description: "Defaults to false if omitted."},
					"bankId":     {Type: "string", Description: "Filter by bank id."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start date; defaults to month-to-date."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end date; defaults to today when month-to-date."},
					"groupBy": {Type: "string", Enum: []string{
						"pfcPrimary",
						"merchant",
						"day",
					}, Description: "Required. Group by category, merchant, or day."},
				},
				Required: []string{"groupBy"},
			},
		},
		{
			Name:        "get_transactions",
			Description: "Return a filtered list of transactions.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Primary category filter."},
					"pending":    {Type: "boolean", Description: "Defaults to false if omitted."},
					"bankId":     {Type: "string", Description: "Filter by bank id."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start date; defaults to month-to-date."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end date; defaults to today when month-to-date."},
					"orderBy":    {Type: "string", Description: "Sort field; defaults to date."},
					"desc":       {Type: "boolean", Description: "Sort descending if true."},
					"limit":      {Type: "integer", Description: "Maximum number of results; defaults to 25."},
				},
			},
		},
	}
}

func systemPrompt(now time.Time) string {
	today := now.Format("2006-01-02")
	weekday := now.Weekday().String()
	return "You are a finance analytics assistant. Use tools for deterministic queries. " +
		"Defaults: pending=false; date range defaults to month-to-date if not provided. " +
		"Do not fabricate data; only answer from tool results. If you did not call a tool, ask a clarification question. " +
		"Today is " + today + " (" + weekday + ", US). " +
		"Important: never include role labels like 'Assistant:' or 'User:' in responses. Respond with the answer only."
}

func strictSystemPrompt(now time.Time) string {
	return systemPrompt(now) + " You must respond with a valid tool call that matches the schema. " +
		"If required information is missing, ask a clarification question instead of calling a tool."
}

func decodeArgs[T any](args map[string]any) (T, error) {
	var out T
	if len(args) == 0 {
		return out, nil
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (s *aiService) applyDefaults(pending **bool, dateFrom **string, dateTo **string) error {
	if *pending == nil {
		*pending = helpers.Ptr(false)
	}

	if (dateFrom == nil || *dateFrom == nil || **dateFrom == "") && (dateTo == nil || *dateTo == nil || **dateTo == "") {
		now := s.clockNow()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		startStr := start.Format("2006-01-02")
		endStr := now.Format("2006-01-02")
		*dateFrom = helpers.Ptr(startStr)
		*dateTo = helpers.Ptr(endStr)
	}

	return nil
}

func validatePrimary(primary *string) error {
	if helpers.Value(primary) == "" {
		return nil
	}
	if taxonomy.IsPFCPrimaryAllowed(*primary) {
		return nil
	}
	return errs.NewValidationError(fmt.Sprintf("invalid pfcPrimary: %s", *primary))
}

func toMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
