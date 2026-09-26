package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type fakeAccountsSvc struct {
	all    []models.Account
	allUID string
	err    error
}

func (f *fakeAccountsSvc) GetAccounts(ctx context.Context, uid, bankID string) ([]models.Account, error) {
	return nil, f.err
}

func (f *fakeAccountsSvc) GetAllAccounts(ctx context.Context, uid string) ([]models.Account, error) {
	f.allUID = uid
	return f.all, f.err
}

func (f *fakeAccountsSvc) SyncAccounts(ctx context.Context, uid, bankID string) (string, error) {
	return "", f.err
}

func newTestAccountsHandler(svc accountsService) *accountsHandlers {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	return &accountsHandlers{
		ResponseHandler: response.New(log),
		AccountsSvc:     svc,
	}
}

func TestListAllAccountsHandler(t *testing.T) {
	svc := &fakeAccountsSvc{all: []models.Account{{AccountID: "a1"}, {AccountID: "a2"}}}
	h := newTestAccountsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.ListAllAccounts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.allUID != "uid-123" {
		t.Fatalf("service called with uid %q, want uid-123", svc.allUID)
	}

	var body struct {
		Data []models.Account `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("got %d accounts, want 2", len(body.Data))
	}
}

func TestListAllAccountsHandlerError(t *testing.T) {
	h := newTestAccountsHandler(&fakeAccountsSvc{err: errors.New("boom")})

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil).WithContext(ctxWithUID(context.Background()))
	rr := httptest.NewRecorder()

	h.ListAllAccounts(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected error status, got %d", rr.Code)
	}
}
