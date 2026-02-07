package store

import (
	"context"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

func TestTransactionQueryWithEmulator(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST not set")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "test-project")
	if err != nil {
		t.Fatalf("firestore client error: %v", err)
	}
	defer client.Close()

	store := NewTransactionStore(client)
	uid := "user"

	now := time.Date(2025, time.January, 15, 10, 0, 0, 0, time.UTC)
	txs := []models.Transaction{
		{
			TransactionID: "t1",
			BankID:        "b1",
			Name:          "Coffee",
			Amount:        3,
			Currency:      "USD",
			Pending:       false,
			Date:          "2025-01-10",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			TransactionID: "t2",
			BankID:        "b1",
			Name:          "Lunch",
			Amount:        12,
			Currency:      "USD",
			Pending:       true,
			Date:          "2025-01-15",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	for _, tx := range txs {
		_, err := client.Collection("users").Doc(uid).Collection("transactions").Doc(tx.TransactionID).Set(ctx, tx)
		if err != nil {
			t.Fatalf("seed transaction error: %v", err)
		}
	}

	pending := false
	dateFrom := "2025-01-12"
	dateTo := "2025-01-20"

	var results []models.Transaction
	err = store.Query(ctx, uid, dto.TransactionQuery{
		Pending:  &pending,
		DateFrom: &dateFrom,
		DateTo:   &dateTo,
	}, func(tx *models.Transaction) error {
		results = append(results, *tx)
		return nil
	})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}

	pending = true
	results = nil
	err = store.Query(ctx, uid, dto.TransactionQuery{
		Pending:  &pending,
		DateFrom: &dateFrom,
		DateTo:   &dateTo,
	}, func(tx *models.Transaction) error {
		results = append(results, *tx)
		return nil
	})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TransactionID != "t2" {
		t.Fatalf("unexpected transaction: %s", results[0].TransactionID)
	}
}
