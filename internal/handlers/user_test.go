package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/middleware"
)

type stubUserService struct {
	called          bool
	ctx             context.Context
	uid, email      string
	first, lastName string
	err             error
}

func (s *stubUserService) CreateUser(ctx context.Context, uid, email, first, last string) error {
	s.called = true
	s.ctx = ctx
	s.uid = uid
	s.email = email
	s.first = first
	s.lastName = last
	return s.err
}

type stubResponseHandler struct {
	writeSuccessCalled bool
	writeSuccessStatus int
	writeSuccessData   any

	handleErrorCalled bool
	handleError       error

	errorWriteCalled bool
	errorWriteStatus int
	errorWriteCode   string
	errorWriteMsg    string
}

func (s *stubResponseHandler) WriteSuccess(w http.ResponseWriter, status int, data any) {
	s.writeSuccessCalled = true
	s.writeSuccessStatus = status
	s.writeSuccessData = data

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"success":true}`))
}

func (s *stubResponseHandler) ErrorWrite(w http.ResponseWriter, status int, code, message string) {
	s.errorWriteCalled = true
	s.errorWriteStatus = status
	s.errorWriteCode = code
	s.errorWriteMsg = message
	w.WriteHeader(status)
}

func (s *stubResponseHandler) HandleError(w http.ResponseWriter, err error) {
	s.handleErrorCalled = true
	s.handleError = err
	w.WriteHeader(http.StatusInternalServerError)
}

func TestCreateUserSuccess(t *testing.T) {
	userSvc := &stubUserService{}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	body := `{"firstname":"Jane","lastname":"Doe"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UIDKey, "uid-123")
	ctx = context.WithValue(ctx, middleware.EmailKey, "jane@example.com")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.CreateUser(rr, req)

	if !userSvc.called {
		t.Fatalf("expected CreateUser to be called on service")
	}
	if userSvc.uid != "uid-123" || userSvc.email != "jane@example.com" {
		t.Fatalf("service received wrong identifiers: uid=%s email=%s", userSvc.uid, userSvc.email)
	}
	if userSvc.first != "Jane" || userSvc.lastName != "Doe" {
		t.Fatalf("service received wrong name: %s %s", userSvc.first, userSvc.lastName)
	}

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("WriteSuccess not called with status 200")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected response status: %d", rr.Code)
	}
}

func TestCreateUserInvalidJSON(t *testing.T) {
	userSvc := &stubUserService{}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("not-json"))
	req = req.WithContext(context.Background())
	rr := httptest.NewRecorder()

	h.CreateUser(rr, req)

	if userSvc.called {
		t.Fatalf("CreateUser should not be called on service when JSON invalid")
	}
	if !resp.handleErrorCalled {
		t.Fatalf("HandleError should be called on invalid JSON")
	}
	if resp.handleError == nil {
		t.Fatalf("HandleError should receive the decode error")
	}
}

func TestCreateUserServiceError(t *testing.T) {
	userSvc := &stubUserService{err: errors.New("service failure")}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	body := `{"firstname":"Jane","lastname":"Doe"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	req = req.WithContext(context.Background())
	rr := httptest.NewRecorder()

	h.CreateUser(rr, req)

	if !userSvc.called {
		t.Fatalf("expected CreateUser to be called on service")
	}

	if !resp.handleErrorCalled {
		t.Fatalf("expected handler to delegate error to ResponseHandler.HandleError")
	}
	if !errors.Is(resp.handleError, userSvc.err) {
		t.Fatalf("unexpected error passed to HandleError: %v", resp.handleError)
	}
	if resp.writeSuccessCalled {
		t.Fatalf("WriteSuccess should not be called on service error")
	}
}
