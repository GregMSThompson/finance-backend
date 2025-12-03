package response

import (
	"log/slog"
	"net/http"
)

type ResponseHandler interface {
	WriteSuccess(w http.ResponseWriter, status int, data any)
	ErrorWrite(w http.ResponseWriter, status int, code, message string)
	HandleError(w http.ResponseWriter, err error)
}

type responseHandler struct {
	Log slog.Logger
}

func New(log slog.Logger) *responseHandler {
	return &responseHandler{Log: log}
}
