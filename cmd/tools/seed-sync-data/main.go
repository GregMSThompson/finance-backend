// seed-sync-data writes test bank, cursor, and transaction records to Firestore.
//
// Usage:
//
//	go run ./cmd/tools/seed-sync-data --project=<gcp-project> --file=<fixture.yaml>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"gopkg.in/yaml.v3"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/store"
)

type fixture struct {
	UserID       string               `yaml:"userId"`
	Bank         fixtureBank          `yaml:"bank"`
	Cursor       fixtureCursor        `yaml:"cursor"`
	Transactions []fixtureTransaction `yaml:"transactions"`
}

type fixtureBank struct {
	BankID      string `yaml:"bankId"`
	Institution string `yaml:"institution"`
	Status      string `yaml:"status"`
}

type fixtureCursor struct {
	BankID string `yaml:"bankId"`
	Cursor string `yaml:"cursor"`
}

type fixtureTransaction struct {
	TransactionID  string   `yaml:"transactionId"`
	BankID         string   `yaml:"bankId"`
	Name           string   `yaml:"name"`
	Amount         float64  `yaml:"amount"`
	Currency       string   `yaml:"currency"`
	Pending        bool     `yaml:"pending"`
	Date           string   `yaml:"date"`
	AuthorizedDate string   `yaml:"authorizedDate"`
	Categories     []string `yaml:"categories"`
	PFCPrimary     string   `yaml:"pfcPrimary"`
	PFCDetailed    string   `yaml:"pfcDetailed"`
	PFCConfidence  string   `yaml:"pfcConfidence"`
	PFCIconURL     string   `yaml:"pfcIconUrl"`
}

func main() {
	project := flag.String("project", os.Getenv("PROJECTID"), "GCP project ID (required)")
	filePath := flag.String("file", "", "Path to YAML fixture file (required)")
	flag.Parse()

	if *project == "" || *filePath == "" {
		flag.Usage()
		log.Fatal("--project and --file are required")
	}

	data, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("read fixture: %v", err)
	}

	fx, err := parseFixture(data)
	if err != nil {
		log.Fatalf("parse fixture: %v", err)
	}

	ctx := context.Background()

	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		log.Fatalf("firestore client: %v", err)
	}
	defer client.Close()

	banks := store.NewBankStore(client, nil)
	txs := store.NewTransactionStore(client)

	bank := &models.Bank{
		BankID:           fx.Bank.BankID,
		Institution:      fx.Bank.Institution,
		Status:           defaultBankStatus(fx.Bank.Status),
		PlaidPublicToken: syntheticPlaidToken(fx.Bank.BankID),
	}
	if err := banks.Create(ctx, fx.UserID, bank); err != nil {
		log.Fatalf("create bank: %v", err)
	}

	if err := txs.SetCursor(ctx, fx.UserID, fx.Cursor.BankID, fx.Cursor.Cursor); err != nil {
		log.Fatalf("set cursor: %v", err)
	}

	modelTxs := make([]models.Transaction, 0, len(fx.Transactions))
	for _, tx := range fx.Transactions {
		modelTxs = append(modelTxs, models.Transaction{
			TransactionID:  tx.TransactionID,
			BankID:         tx.BankID,
			Name:           tx.Name,
			Amount:         tx.Amount,
			Currency:       tx.Currency,
			Pending:        tx.Pending,
			Date:           tx.Date,
			AuthorizedDate: tx.AuthorizedDate,
			Categories:     tx.Categories,
			PFCPrimary:     tx.PFCPrimary,
			PFCDetailed:    tx.PFCDetailed,
			PFCConfidence:  tx.PFCConfidence,
			PFCIconURL:     tx.PFCIconURL,
		})
	}

	if err := txs.UpsertBatch(ctx, fx.UserID, modelTxs); err != nil {
		log.Fatalf("upsert transactions: %v", err)
	}

	fmt.Printf("sync data seeded\n")
	fmt.Printf("  project:      %s\n", *project)
	fmt.Printf("  userId:       %s\n", fx.UserID)
	fmt.Printf("  bankId:       %s\n", fx.Bank.BankID)
	fmt.Printf("  cursorBankId: %s\n", fx.Cursor.BankID)
	fmt.Printf("  transactions: %d\n", len(modelTxs))
}

func parseFixture(data []byte) (*fixture, error) {
	var fx fixture
	if err := yaml.Unmarshal(data, &fx); err != nil {
		return nil, err
	}
	if err := validateFixture(&fx); err != nil {
		return nil, err
	}
	return &fx, nil
}

func validateFixture(fx *fixture) error {
	if fx.UserID == "" {
		return errors.New("userId is required")
	}
	if fx.Bank.BankID == "" {
		return errors.New("bank.bankId is required")
	}
	if fx.Bank.Institution == "" {
		return errors.New("bank.institution is required")
	}
	if fx.Cursor.BankID == "" {
		return errors.New("cursor.bankId is required")
	}
	if fx.Cursor.Cursor == "" {
		return errors.New("cursor.cursor is required")
	}
	if fx.Cursor.BankID != fx.Bank.BankID {
		return fmt.Errorf("cursor.bankId %q must match bank.bankId %q", fx.Cursor.BankID, fx.Bank.BankID)
	}

	for i, tx := range fx.Transactions {
		if tx.TransactionID == "" {
			return fmt.Errorf("transactions[%d].transactionId is required", i)
		}
		if tx.BankID == "" {
			return fmt.Errorf("transactions[%d].bankId is required", i)
		}
		if tx.BankID != fx.Bank.BankID {
			return fmt.Errorf("transactions[%d].bankId %q must match bank.bankId %q", i, tx.BankID, fx.Bank.BankID)
		}
		if tx.Name == "" {
			return fmt.Errorf("transactions[%d].name is required", i)
		}
		if tx.Currency == "" {
			return fmt.Errorf("transactions[%d].currency is required", i)
		}
		if tx.Date == "" {
			return fmt.Errorf("transactions[%d].date is required", i)
		}
	}

	return nil
}

func defaultBankStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "active"
	}
	return status
}

func syntheticPlaidToken(bankID string) string {
	return "test-access-token-" + bankID
}
