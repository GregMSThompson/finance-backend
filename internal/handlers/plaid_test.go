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

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// fakes implementing handler interfaces
type fakePlaidSvc struct {
	linkToken string
	bankID    string
	syncRes   dto.PlaidServiceSyncResult
	err       error

	gotExchange struct {
		uid    string
		pubTok string
		inst   string
	}
	gotSync struct {
		uid    string
		bankID *string
	}
}

func (f *fakePlaidSvc) CreateLinkToken(ctx context.Context, uid string) (string, error) {
	return f.linkToken, f.err
}
func (f *fakePlaidSvc) ExchangePublicToken(ctx context.Context, uid, publicToken, institutionName string) (string, error) {
	f.gotExchange.uid = uid
	f.gotExchange.pubTok = publicToken
	f.gotExchange.inst = institutionName
	return f.bankID, f.err
}
func (f *fakePlaidSvc) SyncTransactions(ctx context.Context, uid string, bankID *string) (dto.PlaidServiceSyncResult, error) {
	f.gotSync.uid = uid
	f.gotSync.bankID = bankID
	return f.syncRes, f.err
}

type fakeBankSvc struct {
	banks []*models.Bank
	err   error
}

func (f *fakeBankSvc) ListBanks(ctx context.Context, uid string) ([]*models.Bank, error) { return f.banks, f.err }
func (f *fakeBankSvc) DeleteBank(ctx context.Context, uid, bankID string) error          { return f.err }

type plaidStubResponseHandler struct {
	handleErrorCalled bool
	handleError       error
}

func (s *plaidStubResponseHandler) WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.WriteHeader(status)
}

func (s *plaidStubResponseHandler) WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.WriteHeader(status)
}

func (s *plaidStubResponseHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	s.handleErrorCalled = true
	s.handleError = err
	w.WriteHeader(http.StatusInternalServerError)
}

// helper to build handler
func newTestPlaidHandler(p *fakePlaidSvc, b *fakeBankSvc) *plaidHandlers {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	deps := &Deps{
		ResponseHandler: response.New(log),
		PlaidSvc:        p,
		BankSvc:         b,
	}
	return NewPlaidHandlers(deps)
}

func newTestPlaidHandlerWithResp(p *fakePlaidSvc, b *fakeBankSvc, resp *plaidStubResponseHandler) *plaidHandlers {
	deps := &Deps{
		ResponseHandler: resp,
		PlaidSvc:        p,
		BankSvc:         b,
	}
	return NewPlaidHandlers(deps)
}

func ctxWithUID(ctx context.Context) context.Context {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	ctx = logger.ToContext(ctx, log)
	return context.WithValue(ctx, middleware.UIDKey, "uid-123")
}

func TestCreateLinkTokenHandler(t *testing.T) {
	p := &fakePlaidSvc{linkToken: "link-abc"}
	h := newTestPlaidHandler(p, &fakeBankSvc{})

	req := httptest.NewRequest(http.MethodPost, "/plaid/link-token", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.CreateLinkToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Success bool
		Data    map[string]string
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Data["linkToken"] != "link-abc" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestLinkBankHandler(t *testing.T) {
	p := &fakePlaidSvc{bankID: "item-1"}
	h := newTestPlaidHandler(p, &fakeBankSvc{})

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

func TestSyncTransactionsHandler(t *testing.T) {
	p := &fakePlaidSvc{syncRes: dto.PlaidServiceSyncResult{BanksSynced: 1}}
	h := newTestPlaidHandler(p, &fakeBankSvc{})

	body := `{"bankId":"item-1"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions/sync", bytes.NewBufferString(body)).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.SyncTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if p.gotSync.uid != "uid-123" || p.gotSync.bankID == nil || *p.gotSync.bankID != "item-1" {
		t.Fatalf("sync called with %+v", p.gotSync)
	}
}

func TestCreateLinkTokenHandlerServiceError(t *testing.T) {
	p := &fakePlaidSvc{err: errors.New("boom")}
	resp := &plaidStubResponseHandler{}
	h := newTestPlaidHandlerWithResp(p, &fakeBankSvc{}, resp)

	req := httptest.NewRequest(http.MethodPost, "/plaid/link-token", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.CreateLinkToken(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestLinkBankHandlerInvalidJSON(t *testing.T) {
	p := &fakePlaidSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestPlaidHandlerWithResp(p, &fakeBankSvc{}, resp)

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
	h := newTestPlaidHandlerWithResp(p, b, resp)

	req := httptest.NewRequest(http.MethodGet, "/banks", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.ListBanks(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestDeleteBankHandlerServiceError(t *testing.T) {
	p := &fakePlaidSvc{}
	b := &fakeBankSvc{err: errors.New("boom")}
	resp := &plaidStubResponseHandler{}
	h := newTestPlaidHandlerWithResp(p, b, resp)

	req := httptest.NewRequest(http.MethodDelete, "/banks/b1", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.DeleteBank(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestSyncTransactionsHandlerInvalidJSON(t *testing.T) {
	p := &fakePlaidSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestPlaidHandlerWithResp(p, &fakeBankSvc{}, resp)

	req := httptest.NewRequest(http.MethodPost, "/transactions/sync", strings.NewReader("not-json")).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.SyncTransactions(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}
