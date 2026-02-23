package services

import (
	"context"
	"errors"
	"strings"
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
	totalCalls        int
	totalArgs         dto.AnalyticsSpendTotalArgs
	totalResp         dto.AnalyticsSpendTotalResult
	totalErr          error
	breakdownCalls    int
	breakdownArgs     dto.AnalyticsSpendBreakdownArgs
	breakdownResp     dto.AnalyticsSpendBreakdownResult
	breakdownErr      error
	transactionsCalls int
	transactionsArgs  dto.AnalyticsTransactionsArgs
	transactionsResp  dto.AnalyticsTransactionsResult
	transactionsErr   error
	comparisonCalls   int
	comparisonArgs    dto.AnalyticsPeriodComparisonArgs
	comparisonResp    dto.AnalyticsPeriodComparisonResult
	comparisonErr     error
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

func (f *fakeAnalyticsClient) GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error) {
	f.transactionsCalls++
	f.transactionsArgs = args
	if f.transactionsErr != nil {
		return dto.AnalyticsTransactionsResult{}, f.transactionsErr
	}
	return f.transactionsResp, nil
}

func (f *fakeAnalyticsClient) GetPeriodComparison(ctx context.Context, uid string, args dto.AnalyticsPeriodComparisonArgs) (dto.AnalyticsPeriodComparisonResult, error) {
	f.comparisonCalls++
	f.comparisonArgs = args
	if f.comparisonErr != nil {
		return dto.AnalyticsPeriodComparisonResult{}, f.comparisonErr
	}
	return f.comparisonResp, nil
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
	svc := NewAIService(vertex, analytics, store, 0)
	svc.clockNow = func() time.Time {
		return time.Date(2025, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", "session", "How much did I spend?")
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
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", "session", "Hi")
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
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", "session", "What is this?")
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
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", "session", "Multi")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if analytics.totalCalls != 1 {
		t.Fatalf("expected spend total call, got %d", analytics.totalCalls)
	}
	if analytics.transactionsCalls != 0 || analytics.breakdownCalls != 0 {
		t.Fatalf("unexpected analytics calls: tx=%d breakdown=%d", analytics.transactionsCalls, analytics.breakdownCalls)
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
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", "session", "How much?")
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
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	_, err := svc.Query(ctx, "user", "session", "Hi")
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(vertex.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(vertex.requests))
	}
}

func TestAIQueryRetriesWithStrictPromptOnMalformedCall(t *testing.T) {
	vertex := &fakeVertexClient{
		errors: []error{
			errs.NewMalformedFunctionCallError(),
		},
		responses: []dto.VertexGenerateResponse{
			{Text: "Recovered"},
		},
	}
	analytics := &fakeAnalyticsClient{}
	store := &fakeAIStore{}
	svc := NewAIService(vertex, analytics, store, 0)

	ctx := helpers.TestCtx()
	resp, err := svc.Query(ctx, "user", "session", "Hello")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if resp.Answer != "Recovered" {
		t.Fatalf("unexpected answer: %q", resp.Answer)
	}
	if len(vertex.requests) != 2 {
		t.Fatalf("expected 2 vertex requests, got %d", len(vertex.requests))
	}
	if !strings.Contains(vertex.requests[1].System, "You must respond with a valid tool call") {
		t.Fatalf("expected strict prompt on retry")
	}
}
