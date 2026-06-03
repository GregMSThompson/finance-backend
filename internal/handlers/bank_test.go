package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type fakeBankSvc struct {
	banks []*models.Bank
	jobID string
	err   error
}

func (f *fakeBankSvc) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) {
	return f.banks, f.err
}
func (f *fakeBankSvc) DeleteBank(ctx context.Context, uid, bankID string) (string, error) {
	return f.jobID, f.err
}

func newTestBankHandler(p *fakePlaidSvc, b *fakeBankSvc) *bankHandlers {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	deps := &Deps{
		ResponseHandler: response.New(log),
		PlaidSvc:        p,
		BankSvc:         b,
	}
	return NewBankHandlers(deps)
}

func newTestBankHandlerWithResp(p *fakePlaidSvc, b *fakeBankSvc, resp *plaidStubResponseHandler) *bankHandlers {
	deps := &Deps{
		ResponseHandler: resp,
		PlaidSvc:        p,
		BankSvc:         b,
	}
	return NewBankHandlers(deps)
}

func TestLinkBankHandler(t *testing.T) {
	p := &fakePlaidSvc{bankID: "item-1"}
	h := newTestBankHandler(p, &fakeBankSvc{})

	body := `{"publicToken":"pub-123","institutionName":"Chase"}`
	req := httptest.NewRequest(http.MethodPost, "/banks", bytes.NewBufferString(body)).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.LinkBank(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if p.gotExchange.uid != "uid-123" || p.gotExchange.pubTok != "pub-123" || p.gotExchange.inst != "Chase" {
		t.Fatalf("exchange called with %+v", p.gotExchange)
	}
}

func TestLinkBankHandlerInvalidJSON(t *testing.T) {
	p := &fakePlaidSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestBankHandlerWithResp(p, &fakeBankSvc{}, resp)

	req := httptest.NewRequest(http.MethodPost, "/banks", strings.NewReader("not-json")).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.LinkBank(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
	if p.gotExchange.uid != "" {
		t.Fatalf("service should not be called on invalid JSON")
	}
}

func TestListBanksHandlerServiceError(t *testing.T) {
	p := &fakePlaidSvc{}
	b := &fakeBankSvc{err: errors.New("boom")}
	resp := &plaidStubResponseHandler{}
	h := newTestBankHandlerWithResp(p, b, resp)

	req := httptest.NewRequest(http.MethodGet, "/banks", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.ListBanks(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestDeleteBankHandler(t *testing.T) {
	p := &fakePlaidSvc{}
	b := &fakeBankSvc{jobID: "job-del"}
	h := newTestBankHandler(p, b)

	req := httptest.NewRequest(http.MethodDelete, "/banks/b1", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.DeleteBank(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Success bool
		Data    map[string]string
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Data["jobId"] != "job-del" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDeleteBankHandlerServiceError(t *testing.T) {
	p := &fakePlaidSvc{}
	b := &fakeBankSvc{err: errors.New("boom")}
	resp := &plaidStubResponseHandler{}
	h := newTestBankHandlerWithResp(p, b, resp)

	req := httptest.NewRequest(http.MethodDelete, "/banks/b1", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.DeleteBank(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}