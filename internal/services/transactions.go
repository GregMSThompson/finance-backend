package services

import (
	"context"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type transactionStore interface {
	Query(ctx context.Context, uid string, q dto.TransactionQuery, handle func(*models.Transaction) error) error
}

type transactionsService struct {
	txs transactionStore
}

func NewTransactionsService(txs transactionStore) *transactionsService {
	return &transactionsService{txs: txs}
}

func (s *transactionsService) ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error) {
	result := dto.TransactionListResult{}

	var txs []models.Transaction
	if err := s.txs.Query(ctx, uid, dto.TransactionQuery{
		Pending:    args.Pending,
		PFCPrimary: args.PFCPrimary,
		BankID:     args.BankID,
		Merchant:   args.Merchant,
		DateFrom:   args.DateFrom,
		DateTo:     args.DateTo,
		OrderBy:    args.OrderBy,
		Desc:       args.Desc,
		Limit:      args.Limit,
	}, func(tx *models.Transaction) error {
		txs = append(txs, *tx)
		return nil
	}); err != nil {
		return result, err
	}

	result.Transactions = txs
	return result, nil
}
