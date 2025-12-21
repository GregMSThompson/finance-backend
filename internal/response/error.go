package response

import (
	"encoding/json"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/errs"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *responseHandler) WriteError(w http.ResponseWriter, status int, code, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Code:    code,
		Message: message,
	})

	h.Log.Error(message, "code", code, "error", err)
}

func (h *responseHandler) HandleError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *errs.NotFoundError:
		h.WriteError(w, http.StatusNotFound, "not_found", e.Message, e)
	case *errs.AlreadyExistsError:
		h.WriteError(w, http.StatusConflict, "already_exists", e.Message, e)
	case *errs.ValidationError:
		h.WriteError(w, http.StatusBadRequest, "invalid_input", e.Message, e)
	default:
		h.WriteError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred", e)
	}
}
