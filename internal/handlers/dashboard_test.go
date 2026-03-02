package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- Stub service ---

type stubDashboardService struct {
	getDashboardWidgets []*models.Widget
	getDashboardErr     error
	addWidgetWidget     *models.Widget
	addWidgetErr        error
	updateWidgetWidget  *models.Widget
	updateWidgetErr     error
	reorderErr          error
	deleteErr           error
	getDataResp         dto.WidgetDataResponse
	getDataErr          error
	lastAddReq          dto.CreateWidgetRequest
	lastUpdateID        string
	lastUpdateReq       dto.UpdateWidgetConfigRequest
	lastDeleteID        string
	lastDataID          string
	lastReorderReq      dto.ReorderWidgetsRequest
}

func (s *stubDashboardService) GetDashboard(_ context.Context, _ string) ([]*models.Widget, error) {
	return s.getDashboardWidgets, s.getDashboardErr
}

func (s *stubDashboardService) AddWidget(_ context.Context, _ string, req dto.CreateWidgetRequest) (*models.Widget, error) {
	s.lastAddReq = req
	return s.addWidgetWidget, s.addWidgetErr
}

func (s *stubDashboardService) UpdateWidgetConfig(_ context.Context, _, widgetID string, req dto.UpdateWidgetConfigRequest) (*models.Widget, error) {
	s.lastUpdateID = widgetID
	s.lastUpdateReq = req
	return s.updateWidgetWidget, s.updateWidgetErr
}

func (s *stubDashboardService) ReorderWidgets(_ context.Context, _ string, req dto.ReorderWidgetsRequest) error {
	s.lastReorderReq = req
	return s.reorderErr
}

func (s *stubDashboardService) DeleteWidget(_ context.Context, _, widgetID string) error {
	s.lastDeleteID = widgetID
	return s.deleteErr
}

func (s *stubDashboardService) GetWidgetData(_ context.Context, _, widgetID string) (dto.WidgetDataResponse, error) {
	s.lastDataID = widgetID
	return s.getDataResp, s.getDataErr
}

// withUID injects a UID into the request context.
func withUID(r *http.Request, uid string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UIDKey, uid)
	return r.WithContext(ctx)
}

// withChiParam injects a chi URL parameter into the request context.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// --- Tests ---

func TestGetDashboard_OK(t *testing.T) {
	svc := &stubDashboardService{
		getDashboardWidgets: []*models.Widget{
			{WidgetID: "w1", Type: dto.WidgetTypeTopSpenders},
		},
	}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetDashboard(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess with 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
}

func TestGetDashboard_ServiceError(t *testing.T) {
	svc := &stubDashboardService{getDashboardErr: errors.New("db failure")}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.GetDashboard(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError to be called")
	}
}

func TestAddWidget_OK(t *testing.T) {
	svc := &stubDashboardService{
		addWidgetWidget: &models.Widget{WidgetID: "w1", Type: dto.WidgetTypeTopSpenders},
	}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	body := `{"type":"topSpenders","visualization":"pie","config":{"dateRange":{"preset":"thisMonth"},"dimension":"category"}}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/widgets", strings.NewReader(body))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.AddWidget(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusCreated {
		t.Fatalf("expected WriteSuccess with 201, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastAddReq.Type != dto.WidgetTypeTopSpenders {
		t.Errorf("unexpected type passed to service: %s", svc.lastAddReq.Type)
	}
}

func TestAddWidget_InvalidJSON(t *testing.T) {
	svc := &stubDashboardService{}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/widgets", strings.NewReader("not-json"))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.AddWidget(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on invalid JSON")
	}
	if resp.writeSuccessCalled {
		t.Fatal("WriteSuccess should not be called on invalid JSON")
	}
}

func TestAddWidget_ServiceError(t *testing.T) {
	svc := &stubDashboardService{addWidgetErr: errs.NewValidationError("config.dateRange is required")}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	body := `{"type":"topSpenders","visualization":"pie"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/widgets", strings.NewReader(body))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.AddWidget(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on service error")
	}
}

func TestUpdateWidgetConfig_OK(t *testing.T) {
	svc := &stubDashboardService{
		updateWidgetWidget: &models.Widget{WidgetID: "w1"},
	}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	body := `{"config":{"dateRange":{"preset":"lastMonth"},"dimension":"merchant","limit":5}}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/widgets/w1", strings.NewReader(body))
	req = withUID(req, "uid1")
	req = withChiParam(req, "widgetId", "w1")
	rr := httptest.NewRecorder()
	h.UpdateWidgetConfig(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastUpdateID != "w1" {
		t.Errorf("expected widgetId=w1, got %s", svc.lastUpdateID)
	}
}

func TestUpdateWidgetConfig_NotFound(t *testing.T) {
	svc := &stubDashboardService{updateWidgetErr: errs.NewNotFoundError("widget not found")}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	body := `{"config":{}}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/widgets/missing", strings.NewReader(body))
	req = withUID(req, "uid1")
	req = withChiParam(req, "widgetId", "missing")
	rr := httptest.NewRecorder()
	h.UpdateWidgetConfig(rr, req)

	if !resp.handleErrorCalled {
		t.Fatal("expected HandleError on not found")
	}
}

func TestReorderWidgets_OK(t *testing.T) {
	svc := &stubDashboardService{}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	body := `{"widgetOrder":[{"widgetId":"w1","position":2},{"widgetId":"w2","position":1}]}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/widgets/reorder", strings.NewReader(body))
	req = withUID(req, "uid1")
	rr := httptest.NewRecorder()
	h.ReorderWidgets(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200")
	}
	if len(svc.lastReorderReq.WidgetOrder) != 2 {
		t.Errorf("expected 2 items in reorder req, got %d", len(svc.lastReorderReq.WidgetOrder))
	}
}

func TestDeleteWidget_OK(t *testing.T) {
	svc := &stubDashboardService{}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/widgets/w1", nil)
	req = withUID(req, "uid1")
	req = withChiParam(req, "widgetId", "w1")
	rr := httptest.NewRecorder()
	h.DeleteWidget(rr, req)

	if !resp.writeSuccessCalled {
		t.Fatal("expected WriteSuccess on delete")
	}
	if svc.lastDeleteID != "w1" {
		t.Errorf("expected widgetId=w1, got %s", svc.lastDeleteID)
	}
}

func TestGetWidgetData_OK(t *testing.T) {
	svc := &stubDashboardService{
		getDataResp: dto.WidgetDataResponse{WidgetID: "w1"},
	}
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: svc})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/widgets/w1/data", nil)
	req = withUID(req, "uid1")
	req = withChiParam(req, "widgetId", "w1")
	rr := httptest.NewRecorder()
	h.GetWidgetData(rr, req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected WriteSuccess 200")
	}
	if svc.lastDataID != "w1" {
		t.Errorf("expected widgetId=w1, got %s", svc.lastDataID)
	}
}

func TestGetWidgetTypes_OK(t *testing.T) {
	resp := &stubResponseHandler{}
	h := NewDashboardHandlers(&Deps{ResponseHandler: resp, DashboardSvc: &stubDashboardService{}})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/widget-types", nil)
	rr := httptest.NewRecorder()
	h.GetWidgetTypes(rr, req)

	if !resp.writeSuccessCalled {
		t.Fatal("expected WriteSuccess")
	}
	catalog, ok := resp.writeSuccessData.([]widgetTypeEntry)
	if !ok {
		t.Fatalf("expected []widgetTypeEntry, got %T", resp.writeSuccessData)
	}
	if len(catalog) == 0 {
		t.Fatal("catalog should not be empty")
	}
	found := false
	for _, e := range catalog {
		if e.Type == dto.WidgetTypeTopSpenders {
			found = true
		}
	}
	if !found {
		t.Error("expected topSpenders in catalog")
	}
}
