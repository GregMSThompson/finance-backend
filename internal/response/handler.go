package response

import (
	"log/slog"
	"net/http"
)

type ResponseHandler interface {
	WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any)
	WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string)
	HandleError(w http.ResponseWriter, r *http.Request, err error)
}

type responseHandler struct {
	Log *slog.Logger
}

func New(log *slog.Logger) *responseHandler {
	return &responseHandler{Log: log}
}
