package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type fakeTransactionsSvc struct {
	lastArgs dto.TransactionListArgs
	result   dto.TransactionListResult
	err      error
}

func (f *fakeTransactionsSvc) ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error) {
	f.lastArgs = args
	return f.result, f.err
}

func newTestTransactionsHandler(svc *fakeTransactionsSvc) *transactionsHandlers {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	deps := &Deps{
		ResponseHandler: response.New(log),
		TransactionsSvc: svc,
	}
	return NewTransactionsHandlers(deps)
}

func newTestTransactionsHandlerWithResp(svc *fakeTransactionsSvc, resp *plaidStubResponseHandler) *transactionsHandlers {
	deps := &Deps{
		ResponseHandler: resp,
		TransactionsSvc: svc,
	}
	return NewTransactionsHandlers(deps)
}

func TestListTransactionsHandlerDefaults(t *testing.T) {
	svc := &fakeTransactionsSvc{
		result: dto.TransactionListResult{
			Transactions: []models.Transaction{{TransactionID: "t1"}},
		},
	}
	h := newTestTransactionsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastArgs.Pending == nil || *svc.lastArgs.Pending != false {
		t.Fatalf("expected pending=false default, got %+v", svc.lastArgs.Pending)
	}
	if svc.lastArgs.OrderBy != "date" || !svc.lastArgs.Desc {
		t.Fatalf("expected default orderBy=date desc, got %s desc=%v", svc.lastArgs.OrderBy, svc.lastArgs.Desc)
	}
	if svc.lastArgs.Limit != listTransactionsDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", listTransactionsDefaultLimit, svc.lastArgs.Limit)
	}
}

func TestListTransactionsHandlerParsesFilters(t *testing.T) {
	svc := &fakeTransactionsSvc{}
	h := newTestTransactionsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/transactions?category=FOOD_RETAIL&category=ENTERTAINMENT&bankId=b1&merchant=amazon&dateFrom=2025-01-01&dateTo=2025-01-31&pending=true&limit=25", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(svc.lastArgs.PFCPrimaries) != 2 || svc.lastArgs.PFCPrimaries[0] != "FOOD_RETAIL" || svc.lastArgs.PFCPrimaries[1] != "ENTERTAINMENT" {
		t.Fatalf("categories mismatch: %+v", svc.lastArgs.PFCPrimaries)
	}
	if svc.lastArgs.BankID == nil || *svc.lastArgs.BankID != "b1" {
		t.Fatalf("bankId mismatch: %+v", svc.lastArgs.BankID)
	}
	if svc.lastArgs.Merchant == nil || *svc.lastArgs.Merchant != "amazon" {
		t.Fatalf("merchant mismatch: %+v", svc.lastArgs.Merchant)
	}
	if svc.lastArgs.DateFrom == nil || *svc.lastArgs.DateFrom != "2025-01-01" {
		t.Fatalf("dateFrom mismatch: %+v", svc.lastArgs.DateFrom)
	}
	if svc.lastArgs.DateTo == nil || *svc.lastArgs.DateTo != "2025-01-31" {
		t.Fatalf("dateTo mismatch: %+v", svc.lastArgs.DateTo)
	}
	if svc.lastArgs.Pending == nil || *svc.lastArgs.Pending != true {
		t.Fatalf("pending mismatch: %+v", svc.lastArgs.Pending)
	}
	if svc.lastArgs.Limit != 25 {
		t.Fatalf("limit mismatch: %d", svc.lastArgs.Limit)
	}
}

func TestListTransactionsHandlerClampsLimit(t *testing.T) {
	svc := &fakeTransactionsSvc{}
	h := newTestTransactionsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/transactions?limit=999", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if svc.lastArgs.Limit != listTransactionsMaxLimit {
		t.Fatalf("expected limit clamped to %d, got %d", listTransactionsMaxLimit, svc.lastArgs.Limit)
	}
}

func TestListTransactionsHandlerInvalidLimit(t *testing.T) {
	svc := &fakeTransactionsSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestTransactionsHandlerWithResp(svc, resp)

	req := httptest.NewRequest(http.MethodGet, "/transactions?limit=abc", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestListTransactionsHandlerInvalidPending(t *testing.T) {
	svc := &fakeTransactionsSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestTransactionsHandlerWithResp(svc, resp)

	req := httptest.NewRequest(http.MethodGet, "/transactions?pending=maybe", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestListTransactionsHandlerPassesCursor(t *testing.T) {
	svc := &fakeTransactionsSvc{}
	h := newTestTransactionsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/transactions?cursor=opaque-token", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if svc.lastArgs.Cursor == nil || *svc.lastArgs.Cursor != "opaque-token" {
		t.Fatalf("cursor not passed through: %+v", svc.lastArgs.Cursor)
	}
}

func TestListTransactionsHandlerServiceError(t *testing.T) {
	svc := &fakeTransactionsSvc{err: errors.New("boom")}
	resp := &plaidStubResponseHandler{}
	h := newTestTransactionsHandlerWithResp(svc, resp)

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}
