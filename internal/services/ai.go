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

var toolNameSet = buildToolNameSet()

type vertexClient interface {
	GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error)
}

type analyticsClient interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
	GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error)
	GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error)
	GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error)
	GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error)
	GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error)
	GetIncomeVsExpenses(ctx context.Context, uid string, args dto.AnalyticsIncomeVsExpensesArgs) (dto.IncomeVsExpensesResult, error)
}

type aiTransactions interface {
	ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error)
}

// aiGetTransactionsArgs decodes Vertex's get_transactions tool call. Vertex
// still emits a single pfcPrimary; we convert to PFCPrimaries before calling
// the transactions service.
type aiGetTransactionsArgs struct {
	Pending    *bool   `json:"pending,omitempty"`
	PFCPrimary *string `json:"pfcPrimary,omitempty"`
	AccountID  *string `json:"accountId,omitempty"`
	Merchant   *string `json:"merchant,omitempty"`
	DateFrom   *string `json:"dateFrom,omitempty"`
	DateTo     *string `json:"dateTo,omitempty"`
	OrderBy    string  `json:"orderBy,omitempty"`
	Desc       bool    `json:"desc,omitempty"`
	Limit      int     `json:"limit,omitempty"`
}

type aiStore interface {
	SaveMessage(ctx context.Context, uid, sessionID string, msg models.AIMessage) error
	ListMessages(ctx context.Context, uid, sessionID string, limit int) ([]models.AIMessage, error)
}

type aiService struct {
	vertex       vertexClient
	analysis     analyticsClient
	transactions aiTransactions
	store        aiStore
	ttl          time.Duration
	clockNow     func() time.Time
}

func NewAIService(vertex vertexClient, analysis analyticsClient, transactions aiTransactions, store aiStore, ttl time.Duration) *aiService {
	return &aiService{
		vertex:       vertex,
		analysis:     analysis,
		transactions: transactions,
		store:        store,
		ttl:          ttl,
		clockNow:     time.Now,
	}
}

func (s *aiService) Query(ctx context.Context, uid string, q dto.AIQueryRequest) (dto.AIQueryResponse, error) {
	log := logger.FromContext(ctx)

	history, err := s.store.ListMessages(ctx, uid, q.SessionID, 20)
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	now := s.clockNow()
	contents := convertMessagesToContents(history, q.Message)
	var lastToolCall *dto.VertexToolCall

	for i := 0; i < 3; i++ {
		req := dto.VertexGenerateRequest{
			System:   systemPrompt(now),
			Contents: contents,
			Tools:    toolSchemas(),
			ToolConfig: &dto.VertexToolConfig{
				// VALIDATED constrains the call format (no Python/code emission)
				// while still letting the model answer in natural language.
				Mode: dto.FunctionCallingModeValidated,
			},
		}

		resp, err := s.generate(ctx, req)
		if err != nil {
			return dto.AIQueryResponse{}, err
		}

		if len(resp.ToolCalls) == 0 {
			return s.finishQuery(ctx, uid, q, resp.Text, lastToolCall)
		}

		if len(resp.ToolCalls) > 1 {
			log.Warn("received multiple tool calls, only processing the first", "count", len(resp.ToolCalls))
		}

		toolCall := resp.ToolCalls[0]

		if !isValidToolName(toolCall.Name) {
			return dto.AIQueryResponse{}, errs.NewValidationError(fmt.Sprintf("model requested unknown tool: %s", toolCall.Name))
		}

		log.Info("executing tool", "tool", toolCall.Name, "call", i+1)

		toolResult, err := s.executeTool(ctx, uid, toolCall)
		if err != nil {
			var validationErr *errs.ValidationError
			switch {
			case errors.As(err, &validationErr):
				// A bad-input error is something the model can fix — reflect it
				// back as the tool result so it can adjust args and retry,
				// rather than aborting the whole query.
				log.Info("reflecting tool validation error to model", "tool", toolCall.Name, "error", err.Error())
				toolResult = dto.VertexToolResult{
					Name:     toolCall.Name,
					Response: map[string]any{"error": err.Error()},
				}
			default:
				return dto.AIQueryResponse{}, fmt.Errorf("failed to execute tool %s: %w", toolCall.Name, err)
			}
		}

		contents = append(contents,
			dto.VertexContent{
				Role:  "model",
				Parts: []dto.VertexPart{{FunctionCall: &toolCall}},
			},
			dto.VertexContent{
				Role:  "user",
				Parts: []dto.VertexPart{{FunctionResponse: &toolResult}},
			},
		)

		lastToolCall = &toolCall
	}

	// Tool call budget exhausted — force a text synthesis response.
	finalResp, err := s.vertex.GenerateContent(ctx, dto.VertexGenerateRequest{
		System:   systemPrompt(now),
		Contents: contents,
		Tools:    toolSchemas(),
		ToolConfig: &dto.VertexToolConfig{
			Mode: dto.FunctionCallingModeNone,
		},
	})
	if err != nil {
		return dto.AIQueryResponse{}, err
	}

	return s.finishQuery(ctx, uid, q, finalResp.Text, lastToolCall)
}

// generate performs a single logical model generation. The caller uses
// VALIDATED mode, which constrains any tool call to a well-formed structure.
// If the model still returns a malformed call (a tier that doesn't fully honor
// the constraint), we stop trying to call and fall back to a natural-language
// reply with Mode NONE. Non-malformed errors propagate immediately.
func (s *aiService) generate(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error) {
	log := logger.FromContext(ctx)

	resp, err := s.vertex.GenerateContent(ctx, req)
	var malformed *errs.MalformedFunctionCallError
	if !errors.As(err, &malformed) {
		return resp, err
	}

	log.Warn("malformed function call under VALIDATED, falling back to text reply",
		"finishMessages", malformed.FinishMessages,
	)
	noneReq := req
	noneReq.ToolConfig = &dto.VertexToolConfig{Mode: dto.FunctionCallingModeNone}
	return s.vertex.GenerateContent(ctx, noneReq)
}

func (s *aiService) finishQuery(ctx context.Context, uid string, q dto.AIQueryRequest, text string, lastToolCall *dto.VertexToolCall) (dto.AIQueryResponse, error) {
	log := logger.FromContext(ctx)

	if err := s.saveMessage(ctx, uid, q.SessionID, models.AIMessage{
		Role:    "user",
		Content: q.Message,
	}); err != nil {
		return dto.AIQueryResponse{}, err
	}

	if text != "" {
		if err := s.saveMessage(ctx, uid, q.SessionID, models.AIMessage{
			Role:    "assistant",
			Content: text,
		}); err != nil {
			return dto.AIQueryResponse{}, err
		}
	}

	out := dto.AIQueryResponse{Answer: text}
	if lastToolCall != nil {
		log.Info("ai query completed", "session_id", q.SessionID, "tool", lastToolCall.Name)
		out.Debug = &dto.AIDebugInfo{
			Tool: lastToolCall.Name,
			Args: lastToolCall.Args,
		}
	} else {
		log.Info("ai query completed", "session_id", q.SessionID)
	}

	return out, nil
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
		raw, err := decodeArgs[aiGetTransactionsArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := s.applyDefaults(&raw.Pending, &raw.DateFrom, &raw.DateTo); err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := validatePrimary(raw.PFCPrimary); err != nil {
			return dto.VertexToolResult{}, err
		}
		if err := s.validateMaxDateRange(raw.DateFrom, raw.DateTo); err != nil {
			return dto.VertexToolResult{}, err
		}
		if raw.Limit <= 0 {
			raw.Limit = defaultTransactionLimit
		}
		result, err := s.transactions.ListTransactions(ctx, uid, dto.TransactionListArgs{
			Pending:      raw.Pending,
			PFCPrimaries: helpers.PrimarySlice(raw.PFCPrimary),
			AccountID:    raw.AccountID,
			Merchant:     raw.Merchant,
			DateFrom:     raw.DateFrom,
			DateTo:       raw.DateTo,
			OrderBy:      raw.OrderBy,
			Desc:         raw.Desc,
			Limit:        raw.Limit,
		})
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		payload, err := toMap(result)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		// The model can't paginate, so surface a plain hasMore flag instead of
		// leaking the opaque cursor into its context.
		delete(payload, "nextCursor")
		payload["hasMore"] = result.NextCursor != nil
		return dto.VertexToolResult{Name: call.Name, Response: payload}, nil
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
	case "get_income_vs_expenses":
		args, err := decodeArgs[dto.AnalyticsIncomeVsExpensesArgs](call.Args)
		if err != nil {
			return dto.VertexToolResult{}, err
		}
		if args.DateFrom == "" || args.DateTo == "" {
			return dto.VertexToolResult{}, errs.NewValidationError("dateFrom and dateTo are required")
		}
		result, err := s.analysis.GetIncomeVsExpenses(ctx, uid, args)
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
					"accountId":  {Type: "string", Description: "Filter by account id."},
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
					"accountId":  {Type: "string", Description: "Filter by account id."},
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
			Name: "get_transactions",
			Description: "Return a filtered list of transactions. Call with no date range to get this month's transactions. " +
				"To find the most recent transaction, set orderBy=date, desc=true, limit=1 — no date range needed. " +
				"For the largest/smallest transactions, set orderBy=amount (desc=true for largest). " +
				"The date range must span at most one year. " +
				"If the result is empty, retry with a wider date range before asking the user.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"pfcPrimary": {Type: "string", Enum: taxonomy.PFCPrimaryList, Description: "Primary category filter."},
					"pending":    {Type: "boolean", Description: "Defaults to false if omitted."},
					"accountId":  {Type: "string", Description: "Filter by account id."},
					"merchant":   {Type: "string", Description: "Partial, case-insensitive merchant name filter."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start date. Defaults to the 1st of the current month."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end date. Defaults to today."},
					"orderBy":    {Type: "string", Enum: sortableTransactionFields(), Description: "Sort field; defaults to date."},
					"desc":       {Type: "boolean", Description: "Sort descending if true. desc=true with orderBy=date gives most recent first; with orderBy=amount gives largest first."},
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
					"dateFrom":  {Type: "string", Description: "YYYY-MM-DD start of the lookback window. Required. Default to 3 months ago."},
					"dateTo":    {Type: "string", Description: "YYYY-MM-DD end of the lookback window. Required. Default to today."},
					"accountId": {Type: "string", Description: "Filter by account id."},
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
					"accountId":   {Type: "string", Description: "Filter by account id."},
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
					"accountId":  {Type: "string", Description: "Filter by account id."},
					"dateFrom":   {Type: "string", Description: "YYYY-MM-DD start of the window. Required. Default to 30 days ago."},
					"dateTo":     {Type: "string", Description: "YYYY-MM-DD end of the window. Required. Default to today."},
				},
				Required: []string{"dimension", "dateFrom", "dateTo"},
			},
		},
		{
			Name: "get_income_vs_expenses",
			Description: "Compare total income against total expenses for a given date range. " +
				"Returns income, expenses, and net (income minus expenses). " +
				"Income is identified by the INCOME category; all other transactions are treated as expenses. " +
				"Always provide dateFrom and dateTo; default to the current month if the user does not specify.",
			Parameters: &dto.VertexSchema{
				Type: "object",
				Properties: map[string]*dto.VertexSchema{
					"dateFrom":  {Type: "string", Description: "YYYY-MM-DD start of the window. Required."},
					"dateTo":    {Type: "string", Description: "YYYY-MM-DD end of the window. Required."},
					"accountId": {Type: "string", Description: "Filter by account id."},
				},
				Required: []string{"dateFrom", "dateTo"},
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
					"accountId":  {Type: "string", Description: "Filter by account id."},
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
		"You may make up to 3 tool calls per turn to gather the data you need. " +
		"Calculate date ranges from natural language (e.g., 'last week', 'this month'). A week is defined as Monday to Sunday. " +
		"All financial data (transactions, amounts, categories) must come from tool results - never fabricate these. " +
		"If a query is ambiguous (e.g., which category?), ask for clarification. " +
		"If a tool returns empty results, automatically retry with a wider date range before responding. Always tell the user what period you searched. " +
		"Defaults: pending=false. Tools with a default date range will apply it automatically when you omit the dates — do not ask the user for a date range you can default. " +
		"Today is " + today + " (" + weekday + ", US)."
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

// defaultTransactionLimit bounds how many rows get_transactions returns to the
// model when it doesn't specify a limit, keeping the tool result out of
// context-blowing territory.
const defaultTransactionLimit = 25

// maxTransactionDateRange caps the window get_transactions may scan. It bounds
// how many docs the read pulls back (the amount sort reads the whole window)
// without a per-doc cap. 366 days accommodates a full leap year.
const maxTransactionDateRange = 366 * 24 * time.Hour

// validateMaxDateRange rejects get_transactions windows wider than
// maxTransactionDateRange. A missing dateTo is treated as today; a missing
// dateFrom is rejected outright, since an open-ended past is unbounded.
func (s *aiService) validateMaxDateRange(dateFrom, dateTo *string) error {
	const layout = "2006-01-02"

	toTime := s.clockNow()
	if dateTo != nil && *dateTo != "" {
		t, err := time.Parse(layout, *dateTo)
		if err != nil {
			return errs.NewValidationError(fmt.Sprintf("invalid dateTo: %s", *dateTo))
		}
		toTime = t
	}

	if dateFrom == nil || *dateFrom == "" {
		return errs.NewValidationError("a dateFrom within one year of dateTo is required")
	}
	fromTime, err := time.Parse(layout, *dateFrom)
	if err != nil {
		return errs.NewValidationError(fmt.Sprintf("invalid dateFrom: %s", *dateFrom))
	}

	if toTime.Sub(fromTime) > maxTransactionDateRange {
		return errs.NewValidationError("date range exceeds the one-year maximum; narrow the range")
	}
	return nil
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

func buildToolNameSet() map[string]bool {
	schemas := toolSchemas()
	set := make(map[string]bool, len(schemas))
	for _, t := range schemas {
		set[t.Name] = true
	}
	return set
}

func isValidToolName(name string) bool {
	return toolNameSet[name]
}
