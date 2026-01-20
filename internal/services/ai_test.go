package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type fakeVertexClient struct {
	responses []dto.VertexGenerateResponse
	requests  []dto.VertexGenerateRequest
}

func (f *fakeVertexClient) GenerateContent(ctx context.Context, req dto.VertexGenerateRequest) (dto.VertexGenerateResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return dto.VertexGenerateResponse{}, errors.New("no responses configured")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type fakeAnalyticsClient struct {
	totalCalls int
	totalArgs  dto.AnalyticsSpendTotalArgs
	totalResp  dto.AnalyticsSpendTotalResult
}

func (f *fakeAnalyticsClient) GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error) {
	f.totalCalls++
	f.totalArgs = args
	return f.totalResp, nil
}

func (f *fakeAnalyticsClient) GetSpendBreakdown(ctx context.Context, uid string, args dto.AnalyticsSpendBreakdownArgs) (dto.AnalyticsSpendBreakdownResult, error) {
	return dto.AnalyticsSpendBreakdownResult{}, nil
}

func (f *fakeAnalyticsClient) GetTransactions(ctx context.Context, uid string, args dto.AnalyticsTransactionsArgs) (dto.AnalyticsTransactionsResult, error) {
	return dto.AnalyticsTransactionsResult{}, nil
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
	svc := NewAIService(vertex, analytics)
	svc.clockNow = func() time.Time {
		return time.Date(2025, time.February, 15, 12, 0, 0, 0, time.UTC)
	}

	resp, err := svc.Query(context.Background(), "user", "How much did I spend?")
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
	svc := NewAIService(vertex, analytics)

	resp, err := svc.Query(context.Background(), "user", "Hi")
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
