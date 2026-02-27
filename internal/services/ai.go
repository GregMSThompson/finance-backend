package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type vertexClient interface {
	GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error)
}

type analyticsClient interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
	GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error)
	GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error)
	GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error)
	GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error)
	GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error)
	GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error)
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
	log := logger.FromContext(ctx)

	history, err := s.store.ListMessages(ctx, uid, sessionID, 8)
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	contents := convertMessagesToContents(history, message)
	req := dto.VertexGenerateRequest{
		System:   systemPrompt(s.clockNow()),
		Contents: contents,
		Tools:    toolSchemas(),
		ToolConfig: &dto.VertexToolConfig{
			Mode: dto.FunctionCallingModeAuto,
		},
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
		// Only save non-empty assistant responses
		if resp.Text != "" {
			if err := s.saveMessage(ctx, uid, sessionID, models.AIMessage{
				Role:    "assistant",
				Content: resp.Text,
			}); err != nil {
				return dto.AIQueryResponse{}, err
			}
		}
		log.Info("ai query completed", "session_id", sessionID)
		return dto.AIQueryResponse{Answer: resp.Text}, nil
	}

	// Handle multiple tool calls (currently only processing the first one)
	if len(resp.ToolCalls) > 1 {
		log.Warn("received multiple tool calls, only processing the first", "count", len(resp.ToolCalls))
	}

	toolCall := resp.ToolCalls[0]

	// Validate tool call name before executing
	if !isValidToolName(toolCall.Name) {
		return dto.AIQueryResponse{}, errs.NewValidationError(fmt.Sprintf("model requested unknown tool: %s", toolCall.Name))
	}

	log.Info("executing tool", "tool", toolCall.Name)

	toolResult, err := s.executeTool(ctx, uid, toolCall)
	if err != nil {
		return dto.AIQueryResponse{}, fmt.Errorf("failed to execute tool %s: %w", toolCall.Name, err)
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

	// For the second request after tool execution, add the tool result to contents
	contentsWithToolResult := append(contents, dto.VertexContent{
		Role: "model",
		Parts: []dto.VertexPart{
			{FunctionCall: &toolCall},
		},
	}, dto.VertexContent{
		Role: "user",
		Parts: []dto.VertexPart{
			{FunctionResponse: &toolResult},
		},
	})

	finalResp, err := s.vertex.GenerateContent(ctx, dto.VertexGenerateRequest{
		System:   systemPrompt(s.clockNow()),
		Contents: contentsWithToolResult,
		Tools:    toolSchemas(),
		ToolConfig: &dto.VertexToolConfig{
			Mode: dto.FunctionCallingModeNone,
		},
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

	log.Info("ai query completed", "session_id", sessionID, "tool", toolCall.Name)
	return dto.AIQueryResponse{
		Answer: finalResp.Text,
		Debug: &dto.AIDebugInfo{
			Tool: toolCall.Name,
			Args: toolCall.Args,
		},
	}, nil
}

func convertMessagesToContents(history []models.AIMessage, currentMessage string) []dto.VertexContent {
	contents := make([]dto.VertexContent, 0, len(history)+1)

	for _, msg := range history {
		switch msg.Role {
		case "user":
			contents = append(contents, dto.VertexContent{
				Role: "user",
				Parts: []dto.VertexPart{
					{Text: &msg.Content},
				},
			})

		case "assistant":
			if msg.Content != "" {
				contents = append(contents, dto.VertexContent{
					Role: "model",
					Parts: []dto.VertexPart{
						{Text: &msg.Content},
					},
				})
			}

		case "tool":
			// Tool calls and results need explicit function call/response parts.
			if msg.ToolName != "" && msg.ToolArgs != nil {
				contents = append(contents, dto.VertexContent{
					Role: "model",
					Parts: []dto.VertexPart{
						{FunctionCall: &dto.VertexToolCall{
							Name: msg.ToolName,
							Args: msg.ToolArgs,
						}},
					},
				})
			}

			if msg.ToolName != "" && msg.ToolResult != nil {
				contents = append(contents, dto.VertexContent{
					Role: "user",
					Parts: []dto.VertexPart{
						{FunctionResponse: &dto.VertexToolResult{
							Name:     msg.ToolName,
							Response: msg.ToolResult,
						}},
					},
				})
			}
		}
	}

	contents = append(contents, dto.VertexContent{
		Role: "user",
		Parts: []dto.VertexPart{
			{Text: &currentMessage},
		},
	})

	return contents
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
		return executeAnalyticsTool(
			ctx,
			uid,
			call,
			func(a *dto.AnalyticsSpendTotalArgs) **bool { return &a.Pending },
			func(a *dto.AnalyticsSpendTotalArgs) **string { return &a.DateFrom },
			func(a *dto.AnalyticsSpendTotalArgs) **string { return &a.DateTo },
			func(a *dto.AnalyticsSpendTotalArgs) *string { return a.PFCPrimary },
			nil,
			s.applyDefaults,
			s.analysis.GetSpendTotal,
		)
	case "get_spend_breakdown":
		return executeAnalyticsTool(
			ctx,
			uid,
			call,
			func(a *dto.AnalyticsSpendBreakdownArgs) **bool { return &a.Pending },
			func(a *dto.AnalyticsSpendBreakdownArgs) **string { return &a.DateFrom },
			func(a *dto.AnalyticsSpendBreakdownArgs) **string { return &a.DateTo },
			func(a *dto.AnalyticsSpendBreakdownArgs) *string { return a.PFCPrimary },
			func(a *dto.AnalyticsSpendBreakdownArgs) error {
				if a.GroupBy == "" {
					return errs.NewValidationError("groupBy is required")
				}
				return nil
			},
			s.applyDefaults,
			s.analysis.GetSpendBreakdown,
		)
	case "get_transactions":
		return executeAnalyticsTool(
			ctx,
			uid,
			call,
			func(a *dto.AnalyticsTransactionsArgs) **bool { return &a.Pending },
			func(a *dto.AnalyticsTransactionsArgs) **string { return &a.DateFrom },
			func(a *dto.AnalyticsTransactionsArgs) **string { return &a.DateTo },
			func(a *dto.AnalyticsTransactionsArgs) *string { return a.PFCPrimary },
			nil,
			s.applyDefaults,
			s.analysis.GetTransactions,
		)
	case "get_period_comparison":
		args, err := decodeArgs[dto.AnalyticsPeriodComparisonArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.Pending == nil {
			args.Pending = helpers.Ptr(false)
		}
		if err := validatePrimary(args.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.CurrentFrom == "" || args.CurrentTo == "" || args.PreviousFrom == "" || args.PreviousTo == "" {
			return dto.VertexToolResult{}, errs.NewValidationError("currentFrom, currentTo, previousFrom, and previousTo are all required")
		}
		result, err := s.analysis.GetPeriodComparison(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	case "get_recurring_transactions":
		args, err := decodeArgs[dto.AnalyticsRecurringArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.DateFrom == "" || args.DateTo == "" {
			return dto.VertexToolResult{}, errs.NewValidationError("dateFrom and dateTo are required")
		}
		result, err := s.analysis.GetRecurringTransactions(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	case "get_moving_average":
		args, err := decodeArgs[dto.AnalyticsMovingAverageArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.DateFrom == "" || args.DateTo == "" {
			return dto.VertexToolResult{}, errs.NewValidationError("dateFrom and dateTo are required")
		}
		if args.Granularity == "" {
			args.Granularity = "month"
		}
		if args.Scope == "" {
			args.Scope = "overall"
		}
		result, err := s.analysis.GetMovingAverage(ctx, uid, args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
	case "get_top_n":
		args, err := decodeArgs[dto.AnalyticsTopNArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.DateFrom == "" || args.DateTo == "" {
			return dto.VertexToolResult{}, errs.NewValidationError("dateFrom and dateTo are required")
		}
		if args.Direction == "" {
			args.Direction = "top"
		}
		if args.Limit == 0 {
			args.Limit = 5
		}
		if err := validatePrimary(args.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		result, err := s.analysis.GetTopN(ctx, uid, args)
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

func executeAnalyticsTool[T any, R any](
	ctx context.Context,
	uid string,
	call dto.VertexToolCall,
	pending func(*T) **bool,
	dateFrom func(*T) **string,
	dateTo func(*T) **string,
	primary func(*T) *string,
	validate func(*T) error,
	applyDefaults func(pending **bool, dateFrom **string, dateTo **string) error,
	exec func(context.Context, string, T) (R, error),
) (dto.VertexToolResult, error) {
	// This helper centralizes shared tool prep (decode, defaults, primary validation) across
	// the analytics tools. It uses small accessors because Go generics can't access struct
	// fields by name, and it needs ** pointers to set default values when optional fields
	// are nil.
	args, err := decodeArgs[T](call.Args)
	if err != nil {
		return dto.VertexToolResult{}, err
	}
	if err := applyDefaults(pending(&args), dateFrom(&args), dateTo(&args)); err != nil {
		return dto.VertexToolResult{}, err
	}
	if err := validatePrimary(primary(&args)); err != nil {
		return dto.VertexToolResult{}, err
	}
	if validate != nil {
		if err := validate(&args); err != nil {
			return dto.VertexToolResult{}, err
		}
	}
	result, err := exec(ctx, uid, args)
	if err != nil {
		return dto.VertexToolResult{}, err
	}
	payload, err := toMap(result)
	if err != nil {
		return dto.VertexToolResult{}, err
	}
	return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
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
					"merchant":   {Type: "string", Description: "Partial, case-insensitive merchant name filter."},
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
					"merchant":   {Type: "string", Description: "Partial, case-insensitive merchant name filter."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start date; defaults to month-to-date."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end date; defaults to today when month-to-date."},
					"orderBy":    {Type: "string", Description: "Sort field; defaults to date."},
					"desc":       {Type: "boolean", Description: "Sort descending if true."},
					"limit":      {Type: "integer", Description: "Maximum number of results; defaults to 25."},
				},
			},
		},
		{
			Name: "get_recurring_transactions",
			Description: "Detect recurring transactions such as subscriptions and regular payments. " +
				"Identifies weekly, biweekly, monthly, and quarterly payments only. " +
				"Cannot detect annual subscriptions — if the user asks about annual payments, inform them this tool cannot detect them. " +
				"Always provide dateFrom and dateTo; default to 3 months ago through today if the user does not specify.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"dateFrom": {Type: "string", Description: "YYYY-MM-DD start of the lookback window. Required. Default to 3 months ago."},
					"dateTo":   {Type: "string", Description: "YYYY-MM-DD end of the lookback window. Required. Default to today."},
					"bankId":   {Type: "string", Description: "Filter by bank id."},
				},
				Required: []string{"dateFrom", "dateTo"},
			},
		},
		{
			Name: "get_moving_average",
			Description: "Calculate average spending over a date range with a time-series breakdown. " +
				"Returns an overall average per time unit (day/week/month) and a series of period totals for trend analysis. " +
				"Always provide dateFrom and dateTo; default to 30 days ago through today if the user does not specify.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"dateFrom":    {Type: "string", Description: "YYYY-MM-DD start of the window. Required. Default to 30 days ago."},
					"dateTo":      {Type: "string", Description: "YYYY-MM-DD end of the window. Required. Default to today."},
					"granularity": {Type: "string", Enum: []string{"day", "week", "month"}, Description: "Time unit for the average and series. Defaults to month."},
					"scope":       {Type: "string", Enum: []string{"overall", "category", "merchant"}, Description: "Group the breakdown by category or merchant. Defaults to overall."},
					"pfcPrimary":  {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Filter by category."},
					"merchant":    {Type: "string", Description: "Partial, case-insensitive merchant name filter."},
					"bankId":      {Type: "string", Description: "Filter by bank id."},
				},
				Required: []string{"dateFrom", "dateTo"},
			},
		},
		{
			Name: "get_top_n",
			Description: "Rank spending by merchant or category, returning the top or bottom N results. " +
				"Use for questions like 'what are my top 5 merchants' or 'which categories do I spend least on'. " +
				"Always provide dateFrom and dateTo; default to 30 days ago through today if the user does not specify.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"dimension":  {Type: "string", Enum: []string{"merchant", "category"}, Description: "Rank by merchant or spending category. Required."},
					"direction":  {Type: "string", Enum: []string{"top", "bottom"}, Description: "Return highest (top) or lowest (bottom) spenders. Defaults to top."},
					"limit":      {Type: "integer", Description: "Number of results to return. Defaults to 5."},
					"minCount":   {Type: "integer", Description: "Minimum transaction count for a result to be included. Optional."},
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Filter to a specific category. Most useful with dimension=merchant."},
					"bankId":     {Type: "string", Description: "Filter by bank id."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start of the window. Required. Default to 30 days ago."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end of the window. Required. Default to today."},
				},
				Required: []string{"dimension", "dateFrom", "dateTo"},
			},
		},
		{
			Name:        "get_period_comparison",
			Description: "Compare spending totals between two explicit time periods with optional grouping.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"currentFrom":  {Type: "string", Description: "YYYY-MM-DD start date of the current period. Required."},
					"currentTo":    {Type: "string", Description: "YYYY-MM-DD end date of the current period. Required."},
					"previousFrom": {Type: "string", Description: "YYYY-MM-DD start date of the previous period. Required."},
					"previousTo":   {Type: "string", Description: "YYYY-MM-DD end date of the previous period. Required."},
					"groupBy": {Type: "string", Enum: []string{
						"pfcPrimary",
						"merchant",
						"day",
					}, Description: "Optional. Group comparison by category, merchant, or day. Omit for totals only."},
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Primary category filter."},
					"pending":    {Type: "boolean", Description: "Defaults to false if omitted."},
					"bankId":     {Type: "string", Description: "Filter by bank id."},
					"merchant":   {Type: "string", Description: "Partial, case-insensitive merchant name filter."},
				},
				Required: []string{"currentFrom", "currentTo", "previousFrom", "previousTo"},
			},
		},
	}
}

func systemPrompt(now time.Time) string {
	today := now.Format("2006-01-02")
	weekday := now.Weekday().String()
	return "You are a finance analytics assistant. Use tools for deterministic queries. " +
		"Make only one tool call per request. For multi-part questions, address the primary question first. " +
		"Calculate date ranges from natural language (e.g., 'last week', 'this month'). A week is defined as Monday to Sunday. " +
		"All financial data (transactions, amounts, categories) must come from tool results - never fabricate these. " +
		"If a query is ambiguous (e.g., which category?), ask for clarification. " +
		"Defaults: pending=false; date range defaults to month-to-date if not provided. " +
		"Today is " + today + " (" + weekday + ", US)."
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

func isValidToolName(name string) bool {
	validTools := map[string]bool{
		"get_spend_total":            true,
		"get_spend_breakdown":        true,
		"get_transactions":           true,
		"get_period_comparison":      true,
		"get_recurring_transactions": true,
		"get_moving_average":         true,
		"get_top_n":                  true,
	}
	return validTools[name]
}
