package response

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *responseHandler) WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(ErrorResponse{
		Code:    code,
		Message: message,
	}); err != nil {
		// Use context logger if encoding fails
		log := logger.FromContext(r.Context())
		log.Error("failed to encode error response", "error", err, "status", status, "code", code)
	}
}

func (h *responseHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	log := logger.FromContext(r.Context())

	switch e := err.(type) {
	case *errs.NotFoundError:
		log.Warn("resource not found", "error", e.Message)
		h.WriteError(w, r, http.StatusNotFound, "not_found", e.Message)

	case *errs.AlreadyExistsError:
		log.Warn("resource already exists", "error", e.Message)
		h.WriteError(w, r, http.StatusConflict, "already_exists", e.Message)

	case *errs.ValidationError:
		log.Warn("validation failed", "error", e.Message)
		h.WriteError(w, r, http.StatusBadRequest, "invalid_input", e.Message)

	case *errs.UnsupportedGroupByError:
		log.Warn("unsupported operation", "error", e.Message)
		h.WriteError(w, r, http.StatusBadRequest, "invalid_input", e.Message)

	case *errs.DatabaseError:
		log.Error("database error",
			"operation", e.Operation,
			"error", e.Message)
		h.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"An error occurred")

	case *errs.ExternalServiceError:
		level := slog.LevelError
		if e.Transient {
			level = slog.LevelWarn
		}
		log.Log(r.Context(), level, "external service error",
			"service", e.Service,
			"transient", e.Transient,
			"error", e.Message)

		status := http.StatusBadGateway
		if e.Transient {
			status = http.StatusServiceUnavailable
		}
		h.WriteError(w, r, status, "service_unavailable",
			"Service temporarily unavailable")

	case *errs.EncryptionError:
		log.Error("encryption error", "error", e.Message)
		h.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"An error occurred")

	default:
		log.Error("unexpected error",
			"error", err,
			"type", fmt.Sprintf("%T", err))
		h.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"An unexpected error occurred")
	}
}
