package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type wellKnownHandlers struct {
	AppleAppID string
}

func NewWellKnownHandlers(deps *Deps) *wellKnownHandlers {
	return &wellKnownHandlers{AppleAppID: deps.AppleAppID}
}

func (h *wellKnownHandlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/apple-app-site-association", h.AppleAppSiteAssociation)
	return r
}

// AppleAppSiteAssociation serves the AASA file Apple fetches to validate
// universal links. If the app ID is not configured the file is unavailable
// (404), so we never publish a bogus mapping.
func (h *wellKnownHandlers) AppleAppSiteAssociation(w http.ResponseWriter, r *http.Request) {
	if h.AppleAppID == "" {
		http.NotFound(w, r)
		return
	}
	resp := dto.AppleAppSiteAssociation{
		Applinks: dto.AppleAppLinks{
			Apps: []string{},
			Details: []dto.AppleAppLinkDetail{
				{AppID: h.AppleAppID, Paths: []string{"/plaid-oauth"}},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
