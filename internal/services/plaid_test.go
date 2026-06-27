package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// --- fakes ---

type fakePlaid struct {
	linkToken       string
	itemID          string
	accessToken     string
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
	created     []*models.Bank
	list        []*models.Bank
	err         error
	findUID     string
	findErr     error
	reauthCalls []struct {
		uid    string
		bankID string
		needs  bool
	}
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
func (f *fakeBankStore) FindUserIDByBankID(ctx context.Context, bankID string) (string, error) {
	return f.findUID, f.findErr
}
func (f *fakeBankStore) SetNeedsReauth(ctx context.Context, uid, bankID string, needs bool) error {
	f.reauthCalls = append(f.reauthCalls, struct {
		uid    string
		bankID string
		needs  bool
	}{uid, bankID, needs})
	return nil
}

type fakeBankSvc struct {
	jobID   string
	err     error
	gotUID  string
	gotBank string
}

func (f *fakeBankSvc) DeleteBank(ctx context.Context, uid, bankID string) (string, error) {
	f.gotUID = uid
	f.gotBank = bankID
	return f.jobID, f.err
}

type fakeTxStore struct {
	cursor    string
	upserted  [][]models.Transaction
	setCursor string
	getErr    error
	upsertErr error
	setCurErr error
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

type fakeJobs struct {
	jobID  string
	err    error
	gotUID string
	gotTyp models.JobType
	gotRaw json.RawMessage
}

func (f *fakeJobs) Submit(ctx context.Context, uid string, jobType models.JobType, params json.RawMessage) (string, error) {
	f.gotUID = uid
	f.gotTyp = jobType
	f.gotRaw = params
	return f.jobID, f.err
}

type fakeAccountSyncer struct {
	jobID  string
	err    error
	gotUID string
	gotBank string
}

func (f *fakeAccountSyncer) SyncAccounts(ctx context.Context, uid, bankID string) (string, error) {
	f.gotUID = uid
	f.gotBank = bankID
	return f.jobID, f.err
}

// --- tests ---

func TestExchangePublicTokenStoresBank(t *testing.T) {
	pl := &fakePlaid{itemID: "item-1", accessToken: "at-123"}
	banks := &fakeBankStore{}
	txs := &fakeTxStore{}
	jobs := &fakeJobs{}

	svc := NewPlaidService(pl, banks, txs, jobs, &fakeBankSvc{}, &fakeAccountSyncer{})

	ctx := helpers.TestCtx()
	_, err := svc.ExchangePublicToken(ctx, "uid-1", dto.LinkBankRequest{PublicToken: "public-xyz", InstitutionName: "Chase"})
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

func TestSyncTransactionsSubmitsJob(t *testing.T) {
	pl := &fakePlaid{}
	banks := &fakeBankStore{}
	txs := &fakeTxStore{}
	jobs := &fakeJobs{jobID: "job-xyz"}

	svc := NewPlaidService(pl, banks, txs, jobs, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	bankID := "item-1"
	got, err := svc.SyncTransactions(ctx, "uid-1", dto.SyncTransactionsRequest{BankID: &bankID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "job-xyz" {
		t.Fatalf("expected jobID 'job-xyz', got %q", got)
	}
	if jobs.gotUID != "uid-1" || jobs.gotTyp != models.JobTypePlaidSync {
		t.Fatalf("submit called with unexpected args: uid=%q type=%q", jobs.gotUID, jobs.gotTyp)
	}
	var params dto.PlaidSyncParams
	if err := json.Unmarshal(jobs.gotRaw, &params); err != nil {
		t.Fatalf("params not valid json: %v", err)
	}
	if params.BankID == nil || *params.BankID != "item-1" {
		t.Fatalf("expected params.BankID=item-1, got %+v", params.BankID)
	}
}

func TestRunSyncUsesCursorAndSetsNewCursor(t *testing.T) {
	pl := &fakePlaid{
		syncPages: []dto.PlaidSyncPage{
			{Transactions: []models.Transaction{{TransactionID: "t1"}}, Cursor: "c1", HasMore: true},
			{Transactions: []models.Transaction{{TransactionID: "t2"}}, Cursor: "c2", HasMore: false},
		},
	}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{cursor: "prev-cursor"}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	now := time.Unix(1000, 0)
	svc.clockNow = func() time.Time { return now }

	ctx := helpers.TestCtx()
	res, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
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

func TestRunSyncPropagatesErrors(t *testing.T) {
	pl := &fakePlaid{}
	banks := &fakeBankStore{err: errors.New("boom")}
	txs := &fakeTxStore{}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestExchangePublicTokenPropagatesExchangeError(t *testing.T) {
	pl := &fakePlaid{exchangeErr: errors.New("plaid down")}
	banks := &fakeBankStore{}
	txs := &fakeTxStore{}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.ExchangePublicToken(ctx, "uid-1", dto.LinkBankRequest{PublicToken: "public-xyz", InstitutionName: "Chase"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(banks.created) != 0 {
		t.Fatalf("bank should not be created on exchange error")
	}
}

func TestExchangePublicTokenPropagatesCreateError(t *testing.T) {
	pl := &fakePlaid{itemID: "item-1", accessToken: "at-123"}
	banks := &fakeBankStore{err: errors.New("create failed")}
	txs := &fakeTxStore{}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.ExchangePublicToken(ctx, "uid-1", dto.LinkBankRequest{PublicToken: "public-xyz", InstitutionName: "Chase"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSyncMissingAccessToken(t *testing.T) {
	pl := &fakePlaid{}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: ""}}}
	txs := &fakeTxStore{}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSyncGetCursorError(t *testing.T) {
	pl := &fakePlaid{}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{getErr: errors.New("get cursor failed")}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSyncPlaidError(t *testing.T) {
	pl := &fakePlaid{syncErr: errors.New("plaid sync failed")}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunSyncUpsertError(t *testing.T) {
	pl := &fakePlaid{
		syncPages: []dto.PlaidSyncPage{
			{Transactions: []models.Transaction{{TransactionID: "t1"}}, Cursor: "c1", HasMore: false},
		},
	}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{upsertErr: errors.New("upsert failed")}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWebhookSyncUpdatesAvailableSubmitsBothJobs(t *testing.T) {
	banks := &fakeBankStore{findUID: "uid-1"}
	accounts := &fakeAccountSyncer{jobID: "acct-job"}

	svc := NewPlaidService(&fakePlaid{}, banks, &fakeTxStore{}, &fakeJobs{jobID: "tx-job"}, &fakeBankSvc{}, accounts)
	ctx := helpers.TestCtx()

	err := svc.HandleWebhook(ctx, dto.PlaidWebhook{
		WebhookType: "TRANSACTIONS",
		WebhookCode: "SYNC_UPDATES_AVAILABLE",
		ItemID:      "item-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accounts.gotUID != "uid-1" || accounts.gotBank != "item-1" {
		t.Fatalf("account sync not called with expected args: uid=%q bank=%q", accounts.gotUID, accounts.gotBank)
	}
}

func TestRunSyncSetCursorError(t *testing.T) {
	pl := &fakePlaid{
		syncPages: []dto.PlaidSyncPage{
			{Transactions: []models.Transaction{{TransactionID: "t1"}}, Cursor: "c1", HasMore: false},
		},
	}
	banks := &fakeBankStore{list: []*models.Bank{{BankID: "item-1", PlaidPublicToken: "at-123"}}}
	txs := &fakeTxStore{setCurErr: errors.New("set cursor failed")}

	svc := NewPlaidService(pl, banks, txs, &fakeJobs{}, &fakeBankSvc{}, &fakeAccountSyncer{})
	ctx := helpers.TestCtx()
	_, err := svc.RunSync(ctx, "uid-1", dto.PlaidSyncParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
