package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/response"
)

type dashboardService interface {
	GetDashboard(ctx context.Context, uid string) ([]*models.Widget, error)
	AddWidget(ctx context.Context, uid string, req dto.CreateWidgetRequest) (*models.Widget, error)
	UpdateWidgetConfig(ctx context.Context, uid, widgetID string, req dto.UpdateWidgetConfigRequest) (*models.Widget, error)
	ReorderWidgets(ctx context.Context, uid string, req dto.ReorderWidgetsRequest) error
	DeleteWidget(ctx context.Context, uid, widgetID string) error
	GetWidgetData(ctx context.Context, uid, widgetID string) (dto.WidgetDataResponse, error)
}

type dashboardHandlers struct {
	ResponseHandler response.ResponseHandler
	DashboardSvc    dashboardService
}

func NewDashboardHandlers(deps *Deps) *dashboardHandlers {
	return &dashboardHandlers{
		ResponseHandler: deps.ResponseHandler,
		DashboardSvc:    deps.DashboardSvc,
	}
}

func (h *dashboardHandlers) DashboardRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetDashboard)
	r.Post("/widgets", h.AddWidget)
	r.Put("/widgets/reorder", h.ReorderWidgets) // must be before /{widgetId}
	r.Put("/widgets/{widgetId}", h.UpdateWidgetConfig)
	r.Delete("/widgets/{widgetId}", h.DeleteWidget)
	r.Get("/widgets/{widgetId}", h.GetWidgetData)
	return r
}

func (h *dashboardHandlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UID(r.Context())
	widgets, err := h.DashboardSvc.GetDashboard(r.Context(), uid)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, widgets)
}

func (h *dashboardHandlers) AddWidget(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	widget, err := h.DashboardSvc.AddWidget(r.Context(), uid, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusCreated, widget)
}

func (h *dashboardHandlers) UpdateWidgetConfig(w http.ResponseWriter, r *http.Request) {
	widgetID := chi.URLParam(r, "widgetId")
	var req dto.UpdateWidgetConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	widget, err := h.DashboardSvc.UpdateWidgetConfig(r.Context(), uid, widgetID, req)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, widget)
}

func (h *dashboardHandlers) ReorderWidgets(w http.ResponseWriter, r *http.Request) {
	var req dto.ReorderWidgetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	uid := middleware.UID(r.Context())
	if err := h.DashboardSvc.ReorderWidgets(r.Context(), uid, req); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}

func (h *dashboardHandlers) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	widgetID := chi.URLParam(r, "widgetId")
	uid := middleware.UID(r.Context())
	if err := h.DashboardSvc.DeleteWidget(r.Context(), uid, widgetID); err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, nil)
}

func (h *dashboardHandlers) GetWidgetData(w http.ResponseWriter, r *http.Request) {
	widgetID := chi.URLParam(r, "widgetId")
	uid := middleware.UID(r.Context())
	data, err := h.DashboardSvc.GetWidgetData(r.Context(), uid, widgetID)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, data)
}

