package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
)

type stubUserService struct {
	called     bool
	ctx        context.Context
	uid, email string
	req        dto.CreateUserRequest
	err        error

	setFCMTokenCalled bool
	setFCMTokenUID    string
	setFCMTokenReq    dto.SetFCMTokenRequest
	setFCMTokenErr    error
}

func (s *stubUserService) CreateUser(ctx context.Context, uid, email string, req dto.CreateUserRequest) error {
	s.called = true
	s.ctx = ctx
	s.uid = uid
	s.email = email
	s.req = req
	return s.err
}

func (s *stubUserService) SetFCMToken(ctx context.Context, uid string, req dto.SetFCMTokenRequest) error {
	s.setFCMTokenCalled = true
	s.setFCMTokenUID = uid
	s.setFCMTokenReq = req
	return s.setFCMTokenErr
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

func (s *stubResponseHandler) WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	s.writeSuccessCalled = true
	s.writeSuccessStatus = status
	s.writeSuccessData = data

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"success":true}`))
}

func (s *stubResponseHandler) WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	s.errorWriteCalled = true
	s.errorWriteStatus = status
	s.errorWriteCode = code
	s.errorWriteMsg = message
	w.WriteHeader(status)
}

func (s *stubResponseHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	s.handleErrorCalled = true
	s.handleError = err
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *stubResponseHandler) WriteTaskSuccess(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *stubResponseHandler) WriteTaskError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.WriteHeader(status)
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
	if userSvc.req.FirstName != "Jane" || userSvc.req.LastName != "Doe" {
		t.Fatalf("service received wrong name: %s %s", userSvc.req.FirstName, userSvc.req.LastName)
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

func TestSetFCMTokenSuccess(t *testing.T) {
	userSvc := &stubUserService{}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	body := `{"token":"fcm-token-abc"}`
	req := httptest.NewRequest(http.MethodPut, "/users/fcm-token", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UIDKey, "uid-123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.SetFCMToken(rr, req)

	if !userSvc.setFCMTokenCalled {
		t.Fatalf("expected SetFCMToken to be called on service")
	}
	if userSvc.setFCMTokenUID != "uid-123" {
		t.Fatalf("service received wrong uid: %s", userSvc.setFCMTokenUID)
	}
	if userSvc.setFCMTokenReq.Token != "fcm-token-abc" {
		t.Fatalf("service received wrong token: %s", userSvc.setFCMTokenReq.Token)
	}
	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("WriteSuccess not called with status 200")
	}
}

func TestSetFCMTokenInvalidJSON(t *testing.T) {
	userSvc := &stubUserService{}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	req := httptest.NewRequest(http.MethodPut, "/users/fcm-token", strings.NewReader("not-json"))
	req = req.WithContext(context.Background())
	rr := httptest.NewRecorder()

	h.SetFCMToken(rr, req)

	if userSvc.setFCMTokenCalled {
		t.Fatalf("SetFCMToken should not be called on service when JSON invalid")
	}
	if !resp.handleErrorCalled {
		t.Fatalf("HandleError should be called on invalid JSON")
	}
}

func TestSetFCMTokenServiceError(t *testing.T) {
	userSvc := &stubUserService{setFCMTokenErr: errors.New("service failure")}
	resp := &stubResponseHandler{}

	h := NewUserHandlers(&Deps{
		ResponseHandler: resp,
		UserSvc:         userSvc,
	})

	body := `{"token":"fcm-token-abc"}`
	req := httptest.NewRequest(http.MethodPut, "/users/fcm-token", strings.NewReader(body))
	req = req.WithContext(context.Background())
	rr := httptest.NewRecorder()

	h.SetFCMToken(rr, req)

	if !userSvc.setFCMTokenCalled {
		t.Fatalf("expected SetFCMToken to be called on service")
	}
	if !resp.handleErrorCalled {
		t.Fatalf("expected handler to delegate error to ResponseHandler.HandleError")
	}
	if !errors.Is(resp.handleError, userSvc.setFCMTokenErr) {
		t.Fatalf("unexpected error passed to HandleError: %v", resp.handleError)
	}
	if resp.writeSuccessCalled {
		t.Fatalf("WriteSuccess should not be called on service error")
	}
}
