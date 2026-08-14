package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type stubAIService struct {
	called bool
	uid    string
	req    dto.AIQueryRequest
	resp   dto.AIQueryResponse
	err    error

	convSessionID string
	convLimit     int
	convResp      dto.AIConversationResponse
	convErr       error
}

func (s *stubAIService) Query(ctx context.Context, uid string, req dto.AIQueryRequest) (dto.AIQueryResponse, error) {
	s.called = true
	s.uid = uid
	s.req = req
	return s.resp, s.err
}

func (s *stubAIService) GetConversation(ctx context.Context, uid, sessionID string, limit int) (dto.AIConversationResponse, error) {
	s.uid = uid
	s.convSessionID = sessionID
	s.convLimit = limit
	return s.convResp, s.convErr
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

func (s *aiStubResponseHandler) WriteTaskSuccess(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *aiStubResponseHandler) WriteTaskError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.WriteHeader(status)
}

func TestAIQueryHandlerSuccess(t *testing.T) {
	aiSvc := &stubAIService{resp: dto.AIQueryResponse{Answer: "ok"}}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	body := `{"sessionId":"s1","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ai/query", strings.NewReader(body))
	ctx := helpers.TestCtx()
	ctx = context.WithValue(ctx, middleware.UIDKey, "uid-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Query(rr, req)

	if !aiSvc.called {
		t.Fatalf("expected AI service to be called")
	}
	if aiSvc.uid != "uid-123" || aiSvc.req.SessionID != "s1" || aiSvc.req.Message != "hello" {
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
	req = req.WithContext(helpers.TestCtx())
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
	req = req.WithContext(helpers.TestCtx())
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
	req = req.WithContext(helpers.TestCtx())
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
	ctx := helpers.TestCtx()
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

func TestGetConversationHandler_DefaultLimit(t *testing.T) {
	aiSvc := &stubAIService{convResp: dto.AIConversationResponse{
		SessionID: "s1",
		Messages:  []dto.AIMessageView{{Role: "user", Content: "hi"}},
	}}
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: aiSvc})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/ai/conversations/s1", nil), "sessionId", "s1"), "uid-123")
	h.GetConversation(httptest.NewRecorder(), req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if aiSvc.uid != "uid-123" || aiSvc.convSessionID != "s1" {
		t.Fatalf("expected uid-123/s1, got %q/%q", aiSvc.uid, aiSvc.convSessionID)
	}
	if aiSvc.convLimit != conversationDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", conversationDefaultLimit, aiSvc.convLimit)
	}
}

func TestGetConversationHandler_LimitOverride(t *testing.T) {
	aiSvc := &stubAIService{}
	h := NewAIHandlers(&Deps{ResponseHandler: &aiStubResponseHandler{}, AISvc: aiSvc})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/ai/conversations/s1?limit=10", nil), "sessionId", "s1"), "uid-123")
	h.GetConversation(httptest.NewRecorder(), req)

	if aiSvc.convLimit != 10 {
		t.Fatalf("expected limit 10, got %d", aiSvc.convLimit)
	}
}

func TestGetConversationHandler_InvalidLimit(t *testing.T) {
	resp := &aiStubResponseHandler{}
	h := NewAIHandlers(&Deps{ResponseHandler: resp, AISvc: &stubAIService{}})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/ai/conversations/s1?limit=0", nil), "sessionId", "s1"), "uid-123")
	h.GetConversation(httptest.NewRecorder(), req)

	if !resp.handleErrorCalled {
		t.Fatal("expected error for invalid limit")
	}
}
