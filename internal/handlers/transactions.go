package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/middleware"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

const (
	listTransactionsDefaultLimit = 50
	listTransactionsMaxLimit     = 200
)

type transactionsService interface {
	ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error)
}

type transactionsHandlers struct {
	ResponseHandler response.ResponseHandler
	TransactionsSvc transactionsService
}

func NewTransactionsHandlers(deps *Deps) *transactionsHandlers {
	return &transactionsHandlers{
		ResponseHandler: deps.ResponseHandler,
		TransactionsSvc: deps.TransactionsSvc,
	}
}

func (h *transactionsHandlers) TransactionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListTransactions)
	return r
}

func (h *transactionsHandlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	args, err := parseListTransactionsArgs(r)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	uid := middleware.UID(r.Context())
	result, err := h.TransactionsSvc.ListTransactions(r.Context(), uid, args)
	if err != nil {
		h.ResponseHandler.HandleError(w, r, err)
		return
	}

	h.ResponseHandler.WriteSuccess(w, r, http.StatusOK, result)
}

// parseListTransactionsArgs translates the GET /transactions query string into
// service arguments and applies transport-level defaults (pending=false,
// desc-by-date, paging limits). Domain validation of category values lives in
// the service.
func parseListTransactionsArgs(r *http.Request) (dto.TransactionListArgs, error) {
	q := r.URL.Query()

	args := dto.TransactionListArgs{
		PFCPrimaries: q["category"],
		BankID:       helpers.OptString(q.Get("bankId")),
		Merchant:     helpers.OptString(q.Get("merchant")),
		DateFrom:     helpers.OptString(q.Get("dateFrom")),
		DateTo:       helpers.OptString(q.Get("dateTo")),
		Cursor:       helpers.OptString(q.Get("cursor")),
		OrderBy:      "date",
		Desc:         true,
		Limit:        listTransactionsDefaultLimit,
	}

	if raw := q.Get("pending"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return args, errs.NewValidationError("pending must be true or false")
		}
		args.Pending = helpers.Ptr(v)
	} else {
		args.Pending = helpers.Ptr(false)
	}

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return args, errs.NewValidationError("limit must be a positive integer")
		}
		if n > listTransactionsMaxLimit {
			n = listTransactionsMaxLimit
		}
		args.Limit = n
	}

	return args, nil
}
