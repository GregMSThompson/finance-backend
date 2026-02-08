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
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type stubAIService struct {
	called    bool
	uid       string
	sessionID string
	message   string
	resp      dto.AIQueryResponse
	err       error
}

func (s *stubAIService) Query(ctx context.Context, uid, sessionID, message string) (dto.AIQueryResponse, error) {
	s.called = true
	s.uid = uid
	s.sessionID = sessionID
	s.message = message
	return s.resp, s.err
}

type aiStubResponseHandler struct {
	writeSuccessCalled bool
	writeSuccessStatus int
	writeSuccessData   any

	handleErrorCalled bool
	handleError       error
}

func (s *aiStubResponseHandler) WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	s.writeSuccessCalled = true
	s.writeSuccessStatus = status
	s.writeSuccessData = data
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (s *aiStubResponseHandler) WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.WriteHeader(status)
}

func (s *aiStubResponseHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	s.handleErrorCalled = true
	s.handleError = err
	w.WriteHeader(http.StatusInternalServerError)
}

func TestAIQueryHandlerSuccess(t *testing.T) {
	aiSvc := &stubAIService{resp: dto.AIQueryResponse{Answer: "ok"}}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	body := `{"sessionId":"s1","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader(body))
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	ctx := logger.ToContext(req.Context(), log)
	ctx = context.WithValue(ctx, middleware.UIDKey, "uid-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if !aiSvc.called {
		t.Fatalf("expected AI service to be called")
	}
	if aiSvc.uid != "uid-123" || aiSvc.sessionID != "s1" || aiSvc.message != "hello" {
		t.Fatalf("service called with unexpected args: %+v", aiSvc)
	}
	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("WriteSuccess not called with status 200")
	}
}

func TestAIQueryHandlerInvalidJSON(t *testing.T) {
	aiSvc := &stubAIService{}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader("not-json"))
	req = req.WithContext(logger.ToContext(req.Context(), slog.New(logger.NewTestHandler(slog.LevelInfo))))
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if aiSvc.called {
		t.Fatalf("service should not be called on invalid JSON")
	}
	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}

func TestAIQueryHandlerMissingMessage(t *testing.T) {
	aiSvc := &stubAIService{}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	body := `{"sessionId":"s1","message":""}`
	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader(body))
	req = req.WithContext(logger.ToContext(req.Context(), slog.New(logger.NewTestHandler(slog.LevelInfo))))
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if aiSvc.called {
		t.Fatalf("service should not be called when message missing")
	}
	var valErr *errs.ValidationError
	if !errors.As(resp.handleError, &valErr) {
		t.Fatalf("expected ValidationError, got %T", resp.handleError)
	}
}

func TestAIQueryHandlerMissingSessionID(t *testing.T) {
	aiSvc := &stubAIService{}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	body := `{"sessionId":"","message":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader(body))
	req = req.WithContext(logger.ToContext(req.Context(), slog.New(logger.NewTestHandler(slog.LevelInfo))))
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if aiSvc.called {
		t.Fatalf("service should not be called when sessionId missing")
	}
	var valErr *errs.ValidationError
	if !errors.As(resp.handleError, &valErr) {
		t.Fatalf("expected ValidationError, got %T", resp.handleError)
	}
}

func TestAIQueryHandlerServiceError(t *testing.T) {
	aiSvc := &stubAIService{err: errors.New("boom")}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	body := `{"sessionId":"s1","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader(body))
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	ctx := logger.ToContext(req.Context(), log)
	ctx = context.WithValue(ctx, middleware.UIDKey, "uid-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if !aiSvc.called {
		t.Fatalf("expected service to be called")
	}
	if !resp.handleErrorCalled {
		t.Fatalf("expected HandleError to be called")
	}
}
