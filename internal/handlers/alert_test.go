package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- Stub service ---

type stubAlertService struct {
	getAlertsResp    []*models.Alert
	getAlertsErr     error
	historyResp      []*models.Notification
	historyErr       error
	createResp       *models.Alert
	createErr        error
	updateResp       *models.Alert
	updateErr        error
	deleteErr        error
	lastCreateReq    dto.CreateAlertRequest
	lastHistoryLimit int
	lastUpdateID     string
	lastUpdateReq    dto.UpdateAlertRequest
	lastDeleteID     string
}

func (s *stubAlertService) GetAlerts(_ context.Context, _ string) ([]*models.Alert, error) {
	return s.getAlertsResp, s.getAlertsErr
}

func (s *stubAlertService) GetAlertHistory(_ context.Context, _ string, limit int) ([]*models.Notification, error) {
	s.lastHistoryLimit = limit
	return s.historyResp, s.historyErr
}

func (s *stubAlertService) CreateAlert(_ context.Context, _ string, req dto.CreateAlertRequest) (*models.Alert, error) {
	s.lastCreateReq = req
	return s.createResp, s.createErr
}

func (s *stubAlertService) UpdateAlert(_ context.Context, _, alertID string, req dto.UpdateAlertRequest) (*models.Alert, error) {
	s.lastUpdateID = alertID
	s.lastUpdateReq = req
	return s.updateResp, s.updateErr
}

func (s *stubAlertService) DeleteAlert(_ context.Context, _, alertID string) error {
	s.lastDeleteID = alertID
	return s.deleteErr
}

// --- Tests ---

func TestGetAlerts_OK(t *testing.T) {
	svc := &stubAlertService{
		getAlertsResp: []*models.Alert{
			{AlertID: "a1", Type: models.AlertTypeSpendThreshold},
		},
	}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetAlerts(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
}

func TestGetAlerts_ServiceError(t *testing.T) {
	svc := &stubAlertService{getAlertsErr: errors.New("db failure")}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetAlerts(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError to be called")
	}
}

func TestGetAlertHistory_OK(t *testing.T) {
	svc := &stubAlertService{
		historyResp: []*models.Notification{
			{NotificationID: "n1", Source: models.NotificationSourceAlert, SourceID: "a1"},
		},
	}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/alerts/history?limit=10", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetAlertHistory(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastHistoryLimit != 10 {
		t.Fatalf("expected limit=10, got %d", svc.lastHistoryLimit)
	}
}

func TestGetAlertHistory_InvalidLimit(t *testing.T) {
	svc := &stubAlertService{}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/alerts/history?limit=bad", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetAlertHistory(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on invalid limit")
	}
}

func TestGetAlertHistory_ServiceError(t *testing.T) {
	svc := &stubAlertService{historyErr: errors.New("db failure")}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/alerts/history", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetAlertHistory(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError to be called")
	}
}

func TestCreateAlert_OK(t *testing.T) {
	svc := &stubAlertService{
		createResp: &models.Alert{AlertID: "a1", Type: models.AlertTypeSpendThreshold},
	}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	body := `{"type":"spend_threshold","enabled":true,"delivery":["email"],"config":{"amountMinor":50000,"window":"30day","dimension":"overall"}}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.CreateAlert(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusCreated {
		t.Fatalf("expected WriteSuccess 201, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastCreateReq.Type != models.AlertTypeSpendThreshold {
		t.Errorf("unexpected type passed to service: %s", svc.lastCreateReq.Type)
	}
}

func TestCreateAlert_InvalidJSON(t *testing.T) {
	svc := &stubAlertService{}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("not-json"))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.CreateAlert(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on invalid JSON")
	}
}

func TestCreateAlert_ServiceError(t *testing.T) {
	svc := &stubAlertService{createErr: errs.NewValidationError("config.amountMinor must be greater than 0")}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	body := `{"type":"spend_threshold","delivery":["email"],"config":{}}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.CreateAlert(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on service error")
	}
}

func TestUpdateAlert_OK(t *testing.T) {
	svc := &stubAlertService{
		updateResp: &models.Alert{AlertID: "a1"},
	}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	body := `{"enabled":false,"delivery":["sms"],"config":{"amountMinor":75000,"window":"7day","dimension":"overall"}}`
	req := httptest.NewRequest(http.MethodPut, "/alerts/a1", strings.NewReader(body))
	req = withUID(req, "uid1")
	req = withChiParam(req, "alertId", "a1")
	rr := httptest.NewRecorder()
	h.UpdateAlert(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastUpdateID != "a1" {
		t.Errorf("expected alertId=a1, got %s", svc.lastUpdateID)
	}
}

func TestUpdateAlert_NotFound(t *testing.T) {
	svc := &stubAlertService{updateErr: errs.NewNotFoundError("alert not found")}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	body := `{"enabled":true,"delivery":["email"],"config":{}}`
	req := httptest.NewRequest(http.MethodPut, "/alerts/missing", strings.NewReader(body))
	req = withUID(req, "uid1")
	req = withChiParam(req, "alertId", "missing")
	rr := httptest.NewRecorder()
	h.UpdateAlert(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on not found")
	}
}

func TestDeleteAlert_OK(t *testing.T) {
	svc := &stubAlertService{}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodDelete, "/alerts/a1", nil)
	req = withUID(req, "uid1")
	req = withChiParam(req, "alertId", "a1")
	rr := httptest.NewRecorder()
	h.DeleteAlert(rr, req)

	if !resp.writeSuccessCalled {
		t.Fatal("expected WriteSuccess on delete")
	}
	if svc.lastDeleteID != "a1" {
		t.Errorf("expected alertId=a1, got %s", svc.lastDeleteID)
	}
}

func TestDeleteAlert_NotFound(t *testing.T) {
	svc := &stubAlertService{deleteErr: errs.NewNotFoundError("alert not found")}
	resp := &stubResponseHandler{}
	h := NewAlertHandlers(&Deps{ResponseHandler: resp, AlertSvc: svc})

	req := httptest.NewRequest(http.MethodDelete, "/alerts/missing", nil)
	req = withUID(req, "uid1")
	req = withChiParam(req, "alertId", "missing")
	rr := httptest.NewRecorder()
	h.DeleteAlert(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on not found")
	}
}
