package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- fakes ---

type fakePlaid struct {
	linkToken      string
	itemID         string
	accessToken    string
	syncPages      []dto.PlaidSyncPage
	createLinkErr  error
	exchangeErr    error
	syncErr        error
	syncCalls      int
	exchangeCalled bool
}

func (f *fakePlaid) CreateLinkToken(ctx context.Context, uid string) (string, error) {
	return f.linkToken, f.createLinkErr
}

func (f *fakePlaid) ExchangePublicToken(ctx context.Context, publicToken string) (string, string, error) {
	f.exchangeCalled = true
	return f.itemID, f.accessToken, f.exchangeErr
}

func (f *fakePlaid) SyncTransactions(ctx context.Context, bankID string, accessToken string, cursor *string) (dto.PlaidSyncPage, error) {
	if f.syncErr != nil {
		return dto.PlaidSyncPage{}, f.syncErr
	}
	if f.syncCalls >= len(f.syncPages) {
		return dto.PlaidSyncPage{}, nil
	}
	page := f.syncPages[f.syncCalls]
	f.syncCalls++
	return page, nil
}

type fakeBankStore struct {
	created []*models.Bank
	list    []*models.Bank
	err     error
}

func (f *fakeBankStore) Create(ctx context.Context, uid string, bank *models.Bank) error {
	if f.err != nil {
		return f.err
	}
	f.created = append(f.created, bank)
	return nil
}
func (f *fakeBankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	return f.list, f.err
}

type fakeTxStore struct {
	cursor     string
	upserted   [][]models.Transaction
	setCursor  string
	getErr     error
	upsertErr  error
	setCurErr  error
}

func (f *fakeTxStore) UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, txs)
	return nil
}
func (f *fakeTxStore) GetCursor(ctx context.Context, uid, bankID string) (string, error) {
	return f.cursor, f.getErr
}
func (f *fakeTxStore) SetCursor(ctx context.Context, uid, bankID, cursor string) error {
	if f.setCurErr != nil {
		return f.setCurErr
	}
	f.setCursor = cursor
	return nil
}

// --- tests ---

func TestExchangePublicTokenStoresBank(t *testing.T) {
	pl := &fakePlaid{itemID: "item-1", accessToken: "at-123"}
	banks := &fakeBankStore{}
	txs := &fakeTxStore{}
	log := slog.New(slog.NewTextHandler(testDiscard{}, nil))

	svc := NewPlaidService(log, pl, banks, txs)

	_, err := svc.ExchangePublicToken(context.Background(), "uid-1", "public-xyz", "Chase")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !pl.exchangeCalled {
		t.Fatal("expected ExchangePublicToken to be called")
	}
	if len(banks.created) != 1 || banks.created[0].Institution != "Chase" {
		t.Fatalf("bank not created with institution, got %+v", banks.created)
	}
	if banks.created[0].PlaidPublicToken != "at-123" {
		t.Fatalf("expected access token to be stored on bank, got %q", banks.created[0].PlaidPublicToken)
	}
}

func TestSyncTransactionsUsesCursorAndSetsNewCursor(t *testing.T) {
	pl := &fakePlaid{
		syncPages: []dto.PlaidSyncPage{
			{Transactions: []models.Transaction{{TransactionID: "t1"}}, Cursor: "c1", HasMore: true},
			{Transactions: []models.Transaction{{TransactionID: "t2"}}, Cursor: "c2", HasMore: false},
		},
	}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{cursor: "prev-cursor"}
	log := slog.New(slog.NewTextHandler(testDiscard{}, nil))

	svc := NewPlaidService(log, pl, banks, txs)
	now := time.Unix(1000, 0)
	svc.clockNow = func() time.Time { return now }

	res, err := svc.SyncTransactions(context.Background(), "uid-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.BanksSynced != 1 {
		t.Fatalf("expected 1 bank synced, got %d", res.BanksSynced)
	}
	if txs.setCursor != "c2" {
		t.Fatalf("expected cursor to be set to c2, got %q", txs.setCursor)
	}
	if len(txs.upserted) != 2 || len(txs.upserted[0]) != 1 || txs.upserted[0][0].TransactionID != "t1" {
		t.Fatalf("unexpected upserts: %+v", txs.upserted)
	}
}

func TestSyncTransactionsPropagatesErrors(t *testing.T) {
	pl := &fakePlaid{}
	banks := &fakeBankStore{err: errors.New("boom")}
	txs := &fakeTxStore{}
	log := slog.New(slog.NewTextHandler(testDiscard{}, nil))

	svc := NewPlaidService(log, pl, banks, txs)
	_, err := svc.SyncTransactions(context.Background(), "uid-1", nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// discard logger output in tests
type testDiscard struct{}

func (testDiscard) Write(p []byte) (int, error) { return len(p), nil }
