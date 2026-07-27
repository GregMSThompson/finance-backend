package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type fakeVertexClient struct {
	responses []dto.VertexGenerateResponse
	errors    []error
	requests  []dto.VertexGenerateRequest
}

func (f *fakeVertexClient) GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.errors) > 0 {
		err := f.errors[0]
		f.errors = f.errors[1:]
		return dto.VertexGenerateResponse{}, err
	}
	if len(f.responses) == 0 {
		return dto.VertexGenerateResponse{}, errors.New("no responses configured")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type fakeAnalyticsClient struct {
	totalCalls       int
	totalArgs        dto.AnalyticsSpendTotalArgs
	totalResp        dto.AnalyticsSpendTotalResult
	totalErr         error
	breakdownCalls   int
	breakdownArgs    dto.AnalyticsSpendBreakdownArgs
	breakdownResp    dto.AnalyticsSpendBreakdownResult
	breakdownErr     error
	comparisonCalls  int
	comparisonArgs   dto.AnalyticsPeriodComparisonArgs
	comparisonResp   dto.AnalyticsPeriodComparisonResult
	comparisonErr    error
	recurringCalls   int
	recurringArgs    dto.AnalyticsRecurringArgs
	recurringResp    dto.RecurringTransactionsResult
	recurringErr     error
	movingAvgCalls   int
	movingAvgArgs    dto.AnalyticsMovingAverageArgs
	movingAvgResp    dto.AnalyticsMovingAverageResult
	movingAvgErr     error
	topNCalls        int
	topNArgs         dto.AnalyticsTopNArgs
	topNResp         dto.AnalyticsTopNResult
	topNErr          error
	incomeVsExpCalls int
	incomeVsExpArgs  dto.AnalyticsIncomeVsExpensesArgs
	incomeVsExpResp  dto.IncomeVsExpensesResult
	incomeVsExpErr   error
}

func (f *fakeAnalyticsClient) GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error) {
	f.totalCalls++
	f.totalArgs = args
	if f.totalErr != nil {
		return dto.AnalyticsSpendTotalResult{}, f.totalErr
	}
	return f.totalResp, nil
}

func (f *fakeAnalyticsClient) GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error) {
	f.breakdownCalls++
	f.breakdownArgs = args
	if f.breakdownErr != nil {
		return dto.AnalyticsSpendBreakdownResult{}, f.breakdownErr
	}
	return f.breakdownResp, nil
}

func (f *fakeAnalyticsClient) GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error) {
	f.comparisonCalls++
	f.comparisonArgs = args
	if f.comparisonErr != nil {
		return dto.AnalyticsPeriodComparisonResult{}, f.comparisonErr
	}
	return f.comparisonResp, nil
}

func (f *fakeAnalyticsClient) GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error) {
	f.recurringCalls++
	f.recurringArgs = args
	if f.recurringErr != nil {
		return dto.RecurringTransactionsResult{}, f.recurringErr
	}
	return f.recurringResp, nil
}

func (f *fakeAnalyticsClient) GetMovingAverage(ctx context.Context, uid string, args dto.AnalyticsMovingAverageArgs) (dto.AnalyticsMovingAverageResult, error) {
	f.movingAvgCalls++
	f.movingAvgArgs = args
	if f.movingAvgErr != nil {
		return dto.AnalyticsMovingAverageResult{}, f.movingAvgErr
	}
	return f.movingAvgResp, nil
}

func (f *fakeAnalyticsClient) GetTopN(ctx context.Context, uid string, args dto.AnalyticsTopNArgs) (dto.AnalyticsTopNResult, error) {
	f.topNCalls++
	f.topNArgs = args
	if f.topNErr != nil {
		return dto.AnalyticsTopNResult{}, f.topNErr
	}
	return f.topNResp, nil
}

func (f *fakeAnalyticsClient) GetIncomeVsExpenses(ctx context.Context, uid string, args dto.AnalyticsIncomeVsExpensesArgs) (dto.IncomeVsExpensesResult, error) {
	f.incomeVsExpCalls++
	f.incomeVsExpArgs = args
	if f.incomeVsExpErr != nil {
		return dto.IncomeVsExpensesResult{}, f.incomeVsExpErr
	}
	return f.incomeVsExpResp, nil
}

type fakeTransactionsLister struct {
	calls int
	args  dto.TransactionListArgs
	resp  dto.TransactionListResult
	err   error
}

func (f *fakeTransactionsLister) ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error) {
	f.calls++
	f.args = args
	if f.err != nil {
		return dto.TransactionListResult{}, f.err
	}
	return f.resp, nil
}

type fakeGoalsService struct {
	createCalls     int
	createSessionID string
	createDef       dto.GoalDefinition
	createResp      *models.Goal
	createErr       error
	updateCalls     int
	updateGoalID    string
	updateUpd       dto.GoalUpdate
	updateResp      *models.Goal
	updateErr       error
	listCalls       int
	listStatuses    []models.GoalStatus
	listResp        []*models.Goal
	listErr         error
	progressCalls   int
	progressGoalID  string
	progressResp    dto.GoalProgress
	progressErr     error
}

func (f *fakeGoalsService) Create(ctx context.Context, uid, sessionID string, def dto.GoalDefinition) (*models.Goal, error) {
	f.createCalls++
	f.createSessionID = sessionID
	f.createDef = def
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createResp, nil
}

func (f *fakeGoalsService) Update(ctx context.Context, uid, goalID string, upd dto.GoalUpdate) (*models.Goal, error) {
	f.updateCalls++
	f.updateGoalID = goalID
	f.updateUpd = upd
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updateResp, nil
}

func (f *fakeGoalsService) List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	f.listCalls++
	f.listStatuses = statuses
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResp, nil
}

func (f *fakeGoalsService) GetProgress(ctx context.Context, uid, goalID string) (dto.GoalProgress, error) {
	f.progressCalls++
	f.progressGoalID = goalID
	if f.progressErr != nil {
		return dto.GoalProgress{}, f.progressErr
	}
	return f.progressResp, nil
}

type fakeAIStore struct {
	messages []models.AIMessage
}

func (f *fakeAIStore) SaveMessage(ctx context.Context, uid, sessionID string, msg models.AIMessage) error {
	f.messages = append(f.messages, msg)
	return nil
}

func (f *fakeAIStore) ListMessages(ctx context.Context, uid, sessionID string, limit int) ([]models.AIMessage, error) {
	if limit > 0 && len(f.messages) > limit {
		return append([]models.AIMessage{}, f.messages[len(f.messages)-limit:]...), nil
	}
	return append([]models.AIMessage{}, f.messages...), nil
}

func TestAIQueryToolFlow(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{
				ToolCalls: []dto.VertexToolCall{
					{Name: "get_spend_total", Args: map[string]any{}},
				},
			},
			{Text: "You spent $5."},
		},
	}
	analytics := &fakeAnalyticsClient{
		totalResp: dto.AnalyticsSpendTotalResult{Total: 5, Currency: "USD"},
	}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)
	svc.clockNow = func() time.Time {
		return time.Date(2025, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "How much did I spend?"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "You spent $5." {
		t.Fatalf("answer mismatch: %q", resp.Answer)
	}
	if analytics.totalCalls != 1 {
		t.Fatalf("expected analytics call, got %d", analytics.totalCalls)
	}
	if helpers.Value(analytics.totalArgs.Pending) != false {
		t.Fatalf("expected pending=false")
	}
	if helpers.Value(analytics.totalArgs.DateFrom) != "2025-02-01" {
		t.Fatalf("dateFrom mismatch: %q", helpers.Value(analytics.totalArgs.DateFrom))
	}
	if helpers.Value(analytics.totalArgs.DateTo) != "2025-02-15" {
		t.Fatalf("dateTo mismatch: %q", helpers.Value(analytics.totalArgs.DateTo))
	}
}

func TestAIQueryNoToolCall(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{{Text: "No tool needed."}},
	}
	analytics := &fakeAnalyticsClient{}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Hi"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "No tool needed." {
		t.Fatalf("answer mismatch: %q", resp.Answer)
	}
	if analytics.totalCalls != 0 {
		t.Fatalf("unexpected analytics calls: %d", analytics.totalCalls)
	}
}

func TestAIQueryUnknownTool(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{
				ToolCalls: []dto.VertexToolCall{
					{Name: "unknown_tool", Args: map[string]any{}},
				},
			},
		},
	}
	analytics := &fakeAnalyticsClient{}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "What is this?"})
	if err == nil {
		t.Fatalf("expected error for unknown tool")
	}
}

func TestAIQueryMultipleToolCallsUsesFirst(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{
				ToolCalls: []dto.VertexToolCall{
					{Name: "get_spend_total", Args: map[string]any{}},
					{Name: "get_transactions", Args: map[string]any{}},
				},
			},
			{Text: "Done"},
		},
	}
	analytics := &fakeAnalyticsClient{
		totalResp: dto.AnalyticsSpendTotalResult{Total: 1, Currency: "USD"},
	}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Multi"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if analytics.totalCalls != 1 {
		t.Fatalf("expected spend total call, got %d", analytics.totalCalls)
	}
	if transactions.calls != 0 || analytics.breakdownCalls != 0 {
		t.Fatalf("unexpected analytics calls: tx=%d breakdown=%d", transactions.calls, analytics.breakdownCalls)
	}
}

func TestAIQueryAnalyticsErrorPropagates(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{
				ToolCalls: []dto.VertexToolCall{
					{Name: "get_spend_total", Args: map[string]any{}},
				},
			},
		},
	}
	analytics := &fakeAnalyticsClient{
		totalErr: errors.New("analytics down"),
	}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "How much?"})
	if err == nil {
		t.Fatalf("expected error from analytics")
	}
}

func TestAIQueryDoesNotRetryOnOtherErrors(t *testing.T) {
	vertex := &fakeVertexClient{
		errors: []error{
			errors.New("other error"),
		},
	}
	analytics := &fakeAnalyticsClient{}
	store := &fakeAIStore{}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, analytics, transactions, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Hi"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(vertex.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(vertex.requests))
	}
}

func TestAIQueryPrimaryUsesValidatedMode(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{{Text: "Hi there."}},
	}
	svc := NewAIService(vertex, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, &fakeGoalsService{}, &fakeAIStore{})

	ctx := helpers.TestCtx()
	if _, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Hello"}); err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if len(vertex.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(vertex.requests))
	}
	if vertex.requests[0].ToolConfig == nil || vertex.requests[0].ToolConfig.Mode != dto.FunctionCallingModeValidated {
		t.Fatalf("expected VALIDATED mode on the primary call, got %+v", vertex.requests[0].ToolConfig)
	}
}

func TestAIQueryMalformedFallsBackToTextReply(t *testing.T) {
	vertex := &fakeVertexClient{
		errors: []error{
			errs.NewMalformedFunctionCallError("print(default_api.get_transactions())"),
		},
		responses: []dto.VertexGenerateResponse{
			{Text: "I couldn't run that query — could you rephrase?"},
		},
	}
	svc := NewAIService(vertex, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, &fakeGoalsService{}, &fakeAIStore{})

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Hello"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "I couldn't run that query — could you rephrase?" {
		t.Fatalf("unexpected answer: %q", resp.Answer)
	}
	if len(vertex.requests) != 2 {
		t.Fatalf("expected 2 vertex requests (validated, none), got %d", len(vertex.requests))
	}
	if vertex.requests[1].ToolConfig == nil || vertex.requests[1].ToolConfig.Mode != dto.FunctionCallingModeNone {
		t.Fatalf("expected NONE mode on the fallback, got %+v", vertex.requests[1].ToolConfig)
	}
}

func TestAIQueryRejectsOverRangeAndReflects(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "get_transactions", Args: map[string]any{
				"dateFrom": "2020-01-01",
				"dateTo":   "2026-01-01",
			}}}},
			{Text: "That range is too wide — showing this month instead."},
		},
	}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, &fakeAnalyticsClient{}, transactions, &fakeGoalsService{}, &fakeAIStore{})
	svc.clockNow = func() time.Time {
		return time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "s", Message: "biggest ever"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if transactions.calls != 0 {
		t.Fatalf("ListTransactions should not run for an over-range request, got %d calls", transactions.calls)
	}
	if len(vertex.requests) != 2 {
		t.Fatalf("expected the validation error to be reflected (2 requests), got %d", len(vertex.requests))
	}
	if resp.Answer != "That range is too wide — showing this month instead." {
		t.Fatalf("unexpected answer: %q", resp.Answer)
	}
}

func TestAIQueryDefaultsTransactionLimit(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "get_transactions", Args: map[string]any{}}}},
			{Text: "Here you go."},
		},
	}
	transactions := &fakeTransactionsLister{}
	svc := NewAIService(vertex, &fakeAnalyticsClient{}, transactions, &fakeGoalsService{}, &fakeAIStore{})
	svc.clockNow = func() time.Time {
		return time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	ctx := helpers.TestCtx()
	if _, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "s", Message: "list"}); err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if transactions.args.Limit != 25 {
		t.Fatalf("expected default limit 25, got %d", transactions.args.Limit)
	}
}

func TestGetTransactionsSurfacesHasMoreNotCursor(t *testing.T) {
	cursor := "opaque"
	transactions := &fakeTransactionsLister{
		resp: dto.TransactionListResult{
			Transactions: []models.Transaction{{TransactionID: "t1"}},
			NextCursor:   &cursor,
		},
	}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, transactions, &fakeGoalsService{}, &fakeAIStore{})
	svc.clockNow = func() time.Time {
		return time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	ctx := helpers.TestCtx()
	res, err := svc.executeTool(ctx, "user", "session", dto.VertexToolCall{
		Name: "get_transactions",
		Args: map[string]any{"dateFrom": "2026-01-01", "dateTo": "2026-01-31"},
	})
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if res.Response["hasMore"] != true {
		t.Fatalf("expected hasMore=true, got %v", res.Response["hasMore"])
	}
	if _, ok := res.Response["nextCursor"]; ok {
		t.Fatalf("nextCursor should be stripped from the model payload")
	}
}

func TestAIQueryMultiToolLoop(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_total", Args: map[string]any{}}}},
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_breakdown", Args: map[string]any{"groupBy": "pfcPrimary"}}}},
			{Text: "Here is your analysis."},
		},
	}
	analytics := &fakeAnalyticsClient{
		totalResp:     dto.AnalyticsSpendTotalResult{Total: 100, Currency: "USD"},
		breakdownResp: dto.AnalyticsSpendBreakdownResult{},
	}
	store := &fakeAIStore{}
	svc := NewAIService(vertex, analytics, &fakeTransactionsLister{}, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Full analysis?"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "Here is your analysis." {
		t.Fatalf("answer mismatch: %q", resp.Answer)
	}
	if analytics.totalCalls != 1 || analytics.breakdownCalls != 1 {
		t.Fatalf("expected 2 tool calls: total=%d breakdown=%d", analytics.totalCalls, analytics.breakdownCalls)
	}
	if len(vertex.requests) != 3 {
		t.Fatalf("expected 3 vertex requests, got %d", len(vertex.requests))
	}
}

func TestAIQueryToolLoopExhausted(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_total", Args: map[string]any{}}}},
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_total", Args: map[string]any{}}}},
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_total", Args: map[string]any{}}}},
			{Text: "Forced synthesis."},
		},
	}
	analytics := &fakeAnalyticsClient{
		totalResp: dto.AnalyticsSpendTotalResult{Total: 5, Currency: "USD"},
	}
	store := &fakeAIStore{}
	svc := NewAIService(vertex, analytics, &fakeTransactionsLister{}, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "Spend?"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "Forced synthesis." {
		t.Fatalf("answer: %q", resp.Answer)
	}
	if len(vertex.requests) != 4 {
		t.Fatalf("expected 4 vertex requests (3 loop + 1 forced), got %d", len(vertex.requests))
	}
	if analytics.totalCalls != 3 {
		t.Fatalf("expected 3 analytics calls, got %d", analytics.totalCalls)
	}
	if vertex.requests[3].ToolConfig == nil || vertex.requests[3].ToolConfig.Mode != dto.FunctionCallingModeNone {
		t.Fatalf("expected NONE mode on forced synthesis request")
	}
}

func TestAIQueryToolMessagesNotSavedToHistory(t *testing.T) {
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "get_spend_total", Args: map[string]any{}}}},
			{Text: "You spent $5."},
		},
	}
	analytics := &fakeAnalyticsClient{totalResp: dto.AnalyticsSpendTotalResult{Total: 5, Currency: "USD"}}
	store := &fakeAIStore{}
	svc := NewAIService(vertex, analytics, &fakeTransactionsLister{}, &fakeGoalsService{}, store)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "session", Message: "How much?"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	for _, msg := range store.messages {
		if msg.Role == "tool" {
			t.Fatalf("tool message should not be saved to history, got %+v", msg)
		}
	}
	if len(store.messages) != 2 {
		t.Fatalf("expected 2 saved messages (user + assistant), got %d", len(store.messages))
	}
	if store.messages[0].Role != "user" || store.messages[1].Role != "assistant" {
		t.Fatalf("expected user then assistant, got %q and %q", store.messages[0].Role, store.messages[1].Role)
	}
}

func TestCreateGoalToolThreadsSessionAndDecodesDefinition(t *testing.T) {
	goals := &fakeGoalsService{createResp: &models.Goal{GoalID: "g1", Name: "Dining"}}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	res, err := svc.executeTool(ctx, "user", "session-42", dto.VertexToolCall{
		Name: "create_goal",
		Args: map[string]any{
			"name":            "Dining",
			"targetValue":     300.0,
			"timeWindow":      "monthly",
			"recurrence":      "recurring",
			"filters":         map[string]any{"pfcPrimary": "FOOD_AND_DRINK"},
			"alertThresholds": map[string]any{"progressPercent": 80.0},
		},
	})
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if goals.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", goals.createCalls)
	}
	if goals.createSessionID != "session-42" {
		t.Fatalf("expected session threaded, got %q", goals.createSessionID)
	}
	if goals.createDef.Type != models.GoalTypeSpendingLimit {
		t.Fatalf("expected type defaulted to spending_limit, got %q", goals.createDef.Type)
	}
	if goals.createDef.TargetValue != 300 || goals.createDef.Filters.PFCPrimary != "FOOD_AND_DRINK" {
		t.Fatalf("definition not decoded: %+v", goals.createDef)
	}
	if goals.createDef.AlertThresholds.ProgressPercent == nil || *goals.createDef.AlertThresholds.ProgressPercent != 80 {
		t.Fatalf("alert threshold not decoded: %+v", goals.createDef.AlertThresholds)
	}
	if res.Response["goalId"] != "g1" {
		t.Fatalf("expected created goal in response, got %v", res.Response["goalId"])
	}
}

func TestCreateGoalToolReflectsValidationError(t *testing.T) {
	goals := &fakeGoalsService{createErr: errs.NewValidationError("targetValue must be greater than 0")}
	vertex := &fakeVertexClient{
		responses: []dto.VertexGenerateResponse{
			{ToolCalls: []dto.VertexToolCall{{Name: "create_goal", Args: map[string]any{
				"name": "Bad", "targetValue": -1.0, "timeWindow": "monthly", "recurrence": "recurring",
			}}}},
			{Text: "That target needs to be positive — how much would you like to cap it at?"},
		},
	}
	svc := NewAIService(vertex, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", dto.AIQueryRequest{SessionID: "s", Message: "cap dining at -1"})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if len(vertex.requests) != 2 {
		t.Fatalf("expected validation error to be reflected (2 requests), got %d", len(vertex.requests))
	}
	if resp.Answer != "That target needs to be positive — how much would you like to cap it at?" {
		t.Fatalf("unexpected answer: %q", resp.Answer)
	}
}

func TestListGoalsToolDefaultsToActiveAndPaused(t *testing.T) {
	goals := &fakeGoalsService{listResp: []*models.Goal{{GoalID: "g1"}}}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	res, err := svc.executeTool(ctx, "user", "s", dto.VertexToolCall{Name: "list_goals", Args: map[string]any{}})
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if len(goals.listStatuses) != 2 ||
		goals.listStatuses[0] != models.GoalStatusActive ||
		goals.listStatuses[1] != models.GoalStatusPaused {
		t.Fatalf("expected default [active paused], got %v", goals.listStatuses)
	}
	if _, ok := res.Response["goals"]; !ok {
		t.Fatalf("expected goals key in response, got %v", res.Response)
	}
}

func TestListGoalsToolPassesStatusFilter(t *testing.T) {
	goals := &fakeGoalsService{}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	if _, err := svc.executeTool(ctx, "user", "s", dto.VertexToolCall{
		Name: "list_goals",
		Args: map[string]any{"status": []any{"completed"}},
	}); err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if len(goals.listStatuses) != 1 || goals.listStatuses[0] != models.GoalStatusCompleted {
		t.Fatalf("expected [completed], got %v", goals.listStatuses)
	}
}

func TestUpdateGoalToolRequiresGoalID(t *testing.T) {
	goals := &fakeGoalsService{}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	_, err := svc.executeTool(ctx, "user", "s", dto.VertexToolCall{
		Name: "update_goal",
		Args: map[string]any{"name": "Renamed"},
	})
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError for missing goalId, got %v", err)
	}
	if goals.updateCalls != 0 {
		t.Fatalf("update should not be called without a goalId, got %d", goals.updateCalls)
	}
}

func TestUpdateGoalToolDecodesPartial(t *testing.T) {
	goals := &fakeGoalsService{updateResp: &models.Goal{GoalID: "g1"}}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	if _, err := svc.executeTool(ctx, "user", "s", dto.VertexToolCall{
		Name: "update_goal",
		Args: map[string]any{"goalId": "g1", "targetValue": 500.0, "status": "paused"},
	}); err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if goals.updateGoalID != "g1" {
		t.Fatalf("expected goalId g1, got %q", goals.updateGoalID)
	}
	if goals.updateUpd.TargetValue == nil || *goals.updateUpd.TargetValue != 500 {
		t.Fatalf("targetValue not decoded: %+v", goals.updateUpd.TargetValue)
	}
	if goals.updateUpd.Status == nil || *goals.updateUpd.Status != models.GoalStatusPaused {
		t.Fatalf("status not decoded: %+v", goals.updateUpd.Status)
	}
	if goals.updateUpd.Name != nil {
		t.Fatalf("name should be nil when omitted, got %v", *goals.updateUpd.Name)
	}
}

func TestGetGoalProgressTool(t *testing.T) {
	goals := &fakeGoalsService{progressResp: dto.GoalProgress{GoalID: "g1", CurrentValue: 120, TargetValue: 300}}
	svc := NewAIService(&fakeVertexClient{}, &fakeAnalyticsClient{}, &fakeTransactionsLister{}, goals, &fakeAIStore{})

	ctx := helpers.TestCtx()
	res, err := svc.executeTool(ctx, "user", "s", dto.VertexToolCall{
		Name: "get_goal_progress",
		Args: map[string]any{"goalId": "g1"},
	})
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if goals.progressGoalID != "g1" {
		t.Fatalf("expected progress for g1, got %q", goals.progressGoalID)
	}
	if res.Response["currentValue"] != 120.0 {
		t.Fatalf("expected currentValue 120 in response, got %v", res.Response["currentValue"])
	}
}
