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
	r.Get("/widget-types", h.GetWidgetTypes)
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

// GetWidgetTypes returns the hardcoded catalog of supported widget types and their valid configurations.
func (h *dashboardHandlers) GetWidgetTypes(w http.ResponseWriter, r *http.Request) {
	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, widgetTypeCatalog)
}

type widgetTypeEntry struct {
	Type           string         `json:"type"`
	Visualizations []string       `json:"visualizations"`
	ConfigOptions  map[string]any `json:"configOptions"`
}

var widgetTypeCatalog = []widgetTypeEntry{
	{
		Type:           dto.WidgetTypeTopSpenders,
		Visualizations: []string{dto.VisPie, dto.VisBar, dto.VisList},
		ConfigOptions: map[string]any{
			"dateRange": "required",
			"dimension": []string{dto.DimensionCategory, dto.DimensionMerchant},
			"limit":     "3-20 (default 10)",
			"category":  "optional PFC primary filter",
			"bankId":    "optional",
		},
	},
	{
		Type:           dto.WidgetTypeSpendingTrend,
		Visualizations: []string{dto.VisLine, dto.VisBar},
		ConfigOptions: map[string]any{
			"window":    []string{dto.Window7Day, dto.Window30Day, dto.Window60Day, dto.Window90Day},
			"dimension": []string{dto.DimensionOverall, dto.DimensionCategory, dto.DimensionMerchant},
			"category":  "optional PFC primary filter",
			"bankId":    "optional",
		},
	},
	{
		Type:           dto.WidgetTypePeriodComparison,
		Visualizations: []string{dto.VisSummary, dto.VisBar},
		ConfigOptions: map[string]any{
			"preset": []string{dto.PeriodMonthOverMonth, dto.PeriodWeekOverWeek, dto.PeriodQuarterOverQuarter, dto.PeriodYearOverYear},
			"bankId": "optional",
		},
	},
	{
		Type:           dto.WidgetTypeLargestTransactions,
		Visualizations: []string{dto.VisList, dto.VisTable},
		ConfigOptions: map[string]any{
			"dateRange": "required",
			"limit":     "5-20 (default 10)",
			"category":  "optional PFC primary filter",
			"bankId":    "optional",
		},
	},
	{
		Type:           dto.WidgetTypeRecurringSubscriptions,
		Visualizations: []string{dto.VisList, dto.VisTable},
		ConfigOptions: map[string]any{
			"bankId": "optional",
		},
	},
}
