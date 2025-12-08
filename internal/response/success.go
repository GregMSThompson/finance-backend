package response

import (
	"encoding/json"
	"net/http"
)

type SuccessEnvelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
}

func (h *responseHandler) WriteSuccess(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := SuccessEnvelope{
		Success: true,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Last-ditch logging; can't return an error now
		h.Log.Error("failed to encode success response", "error", err)
	}
}
