package handlers

import (
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
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
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
func (f *fakePlaidSvc) ExchangePublicToken(ctx context.Context, uid string, req dto.LinkBankRequest) (string, error) {
	f.gotExchange.uid = uid
	f.gotExchange.pubTok = req.PublicToken
	f.gotExchange.inst = req.InstitutionName
	return f.bankID, f.err
}
func (f *fakePlaidSvc) SyncTransactions(ctx context.Context, uid string, req dto.SyncTransactionsRequest) (dto.PlaidServiceSyncResult, error) {
	f.gotSync.uid = uid
	f.gotSync.bankID = req.BankID
	return f.syncRes, f.err
}

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

func (s *plaidStubResponseHandler) WriteTaskSuccess(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *plaidStubResponseHandler) WriteTaskError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.WriteHeader(status)
}

// helper to build handler
func newTestPlaidHandler(p *fakePlaidSvc) *plaidHandlers {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	deps := &Deps{
		ResponseHandler: response.New(log),
		PlaidSvc:        p,
	}
	return NewPlaidHandlers(deps)
}

func newTestPlaidHandlerWithResp(p *fakePlaidSvc, resp *plaidStubResponseHandler) *plaidHandlers {
	deps := &Deps{
		ResponseHandler: resp,
		PlaidSvc:        p,
	}
	return NewPlaidHandlers(deps)
}

func ctxWithUID(ctx context.Context) context.Context {
	ctx = helpers.TestCtx()
	return context.WithValue(ctx, middleware.UIDKey, "uid-123")
}

func TestCreateLinkTokenHandler(t *testing.T) {
	p := &fakePlaidSvc{linkToken: "link-abc"}
	h := newTestPlaidHandler(p)

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

func TestSyncTransactionsHandler(t *testing.T) {
	p := &fakePlaidSvc{syncRes: dto.PlaidServiceSyncResult{BanksSynced: 1}}
	h := newTestPlaidHandler(p)

	body := `{"bankId":"item-1"}`
	req := httptest.NewRequest(http.MethodPost, "/plaid/sync", strings.NewReader(body)).WithContext(ctxWithUID(context.Background()))
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
	h := newTestPlaidHandlerWithResp(p, resp)

	req := httptest.NewRequest(http.MethodPost, "/plaid/link-token", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.CreateLinkToken(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestSyncTransactionsHandlerInvalidJSON(t *testing.T) {
	p := &fakePlaidSvc{}
	resp := &plaidStubResponseHandler{}
	h := newTestPlaidHandlerWithResp(p, resp)

	req := httptest.NewRequest(http.MethodPost, "/plaid/sync", strings.NewReader("not-json")).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.SyncTransactions(rr, req)

	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}