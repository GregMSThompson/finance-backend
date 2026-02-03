package response

import (
	"encoding/json"
	"net/http"

	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type SuccessEnvelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
}

func (h *responseHandler) WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := SuccessEnvelope{
		Success: true,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Use context logger with full request context
		log := logger.FromContext(r.Context())
		log.Error("failed to encode success response", "error", err, "status", status)
	}
}
