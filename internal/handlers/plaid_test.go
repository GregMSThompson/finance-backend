package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
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

// helper to build handler
func newTestPlaidHandler(p *fakePlaidSvc, b *fakeBankSvc) *plaidHandlers {
	log := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	deps := &Deps{
		ResponseHandler: response.New(log),
		PlaidSvc:        p,
		BankSvc:         b,
	}
	return NewPlaidHandlers(deps)
}

func ctxWithUID(ctx context.Context) context.Context {
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

// discard logger output in tests
type testDiscard struct{}

func (testDiscard) Write(p []byte) (int, error) { return len(p), nil }
