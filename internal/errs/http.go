package errs

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorHandler interface {
	Write(w http.ResponseWriter, status int, code, message string)
	HandleError(w http.ResponseWriter, err error)
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorHandler struct {
	Log slog.Logger
}

func NewErrorHandler(log slog.Logger) *errorHandler {
	return &errorHandler{Log: log}
}

func (h *errorHandler) Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Code:    code,
		Message: message,
	})

	h.Log.Error(message, "code", code)
}

func (h *errorHandler) HandleError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *NotFoundError:
		h.Write(w, http.StatusNotFound, "not_found", e.Message)
	case *AlreadyExistsError:
		h.Write(w, http.StatusConflict, "already_exists", e.Message)
	case *ValidationError:
		h.Write(w, http.StatusBadRequest, "invalid_input", e.Message)
	default:
		h.Write(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}
