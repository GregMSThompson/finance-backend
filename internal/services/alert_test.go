package services

import (
	"context"
	"errors"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- Fake store ---

type fakeAlertStore struct {
	alerts    map[string]*models.Alert
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

type fakeAlertEventStore struct {
	events        []*models.AlertEvent
	listRecentErr error
	lastLimit     int
}

func newFakeAlertStore() *fakeAlertStore {
	return &fakeAlertStore{alerts: make(map[string]*models.Alert)}
}

func (f *fakeAlertStore) Create(_ context.Context, _ string, a *models.Alert) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.alerts[a.AlertID] = a
	return nil
}

func (f *fakeAlertStore) Get(_ context.Context, _, alertID string) (*models.Alert, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	a, ok := f.alerts[alertID]
	if !ok {
		return nil, errs.NewNotFoundError("alert not found")
	}
	return a, nil
}

func (f *fakeAlertStore) List(_ context.Context, _ string, _ bool) ([]*models.Alert, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*models.Alert, 0, len(f.alerts))
	for _, a := range f.alerts {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeAlertStore) Update(_ context.Context, _ string, a *models.Alert) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.alerts[a.AlertID] = a
	return nil
}

func (f *fakeAlertStore) Delete(_ context.Context, _, alertID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.alerts, alertID)
	return nil
}

func (f *fakeAlertEventStore) ListRecent(_ context.Context, _ string, limit int) ([]*models.AlertEvent, error) {
	f.lastLimit = limit
	if f.listRecentErr != nil {
		return nil, f.listRecentErr
	}
	return f.events, nil
}

// --- Helpers ---

func newAlertSvc(store alertStore) *alertService {
	return NewAlertService(store, &fakeAlertEventStore{})
}

func newAlertSvcWithEventStore(store alertStore, events alertEventStore) *alertService {
	return NewAlertService(store, events)
}

func validSpendThresholdReq() dto.CreateAlertRequest {
	return dto.CreateAlertRequest{
		Type:     models.AlertTypeSpendThreshold,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryEmail},
		Config: models.AlertConfig{
			AmountMinor: 50000,
			Window:      dto.Window30Day,
			Dimension:   dto.DimensionOverall,
		},
	}
}

func isValidationError(err error) bool {
	var ve *errs.ValidationError
	return errors.As(err, &ve)
}

func isNotFoundError(err error) bool {
	var nfe *errs.NotFoundError
	return errors.As(err, &nfe)
}

// --- spend_threshold tests ---

func TestCreateAlert_SpendThreshold_Overall(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	a, err := svc.CreateAlert(context.Background(), "uid1", validSpendThresholdReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.AlertID == "" {
		t.Error("expected AlertID to be set")
	}
	if a.Type != models.AlertTypeSpendThreshold {
		t.Errorf("unexpected type: %s", a.Type)
	}
}

func TestCreateAlert_SpendThreshold_CategoryDimension(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Dimension = dto.DimensionCategory
	req.Config.Category = "Food and Drink"
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlert_SpendThreshold_CategoryMissingCategory(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Dimension = dto.DimensionCategory
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreateAlert_SpendThreshold_MerchantDimension(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Dimension = dto.DimensionMerchant
	req.Config.Merchant = "Amazon"
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlert_SpendThreshold_MerchantMissingMerchant(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Dimension = dto.DimensionMerchant
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreateAlert_SpendThreshold_InvalidDimension(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Dimension = "invalid"
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreateAlert_SpendThreshold_ZeroAmount(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.AmountMinor = 0
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreateAlert_SpendThreshold_InvalidWindow(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Config.Window = "week"
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

// --- large_transaction tests ---

func TestCreateAlert_LargeTransaction_Valid(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeLargeTransaction,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryPush},
		Config:   models.AlertConfig{AmountMinor: 15000},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlert_LargeTransaction_ZeroAmount(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeLargeTransaction,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryPush},
		Config:   models.AlertConfig{},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

// --- subscription_increase tests ---

func TestCreateAlert_SubscriptionIncrease_Valid(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeSubscriptionIncrease,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryEmail},
		Config:   models.AlertConfig{},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlert_SubscriptionIncrease_WithMerchantFilter(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeSubscriptionIncrease,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryEmail},
		Config:   models.AlertConfig{Merchant: "Netflix"},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- income_received tests ---

func TestCreateAlert_IncomeReceived_Valid(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeIncomeReceived,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryPush, models.DeliveryEmail},
		Config:   models.AlertConfig{},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAlertHistory_DefaultLimit(t *testing.T) {
	store := &fakeAlertEventStore{
		events: []*models.AlertEvent{{EventID: "e1"}},
	}
	svc := newAlertSvcWithEventStore(newFakeAlertStore(), store)

	events, err := svc.GetAlertHistory(context.Background(), "uid1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if store.lastLimit != defaultAlertHistoryLimit {
		t.Fatalf("expected default limit %d, got %d", defaultAlertHistoryLimit, store.lastLimit)
	}
}

func TestGetAlertHistory_MaxLimit(t *testing.T) {
	store := &fakeAlertEventStore{}
	svc := newAlertSvcWithEventStore(newFakeAlertStore(), store)

	_, err := svc.GetAlertHistory(context.Background(), "uid1", maxAlertHistoryLimit+50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastLimit != maxAlertHistoryLimit {
		t.Fatalf("expected max limit %d, got %d", maxAlertHistoryLimit, store.lastLimit)
	}
}

func TestGetAlertHistory_StoreError(t *testing.T) {
	store := &fakeAlertEventStore{listRecentErr: errors.New("db failure")}
	svc := newAlertSvcWithEventStore(newFakeAlertStore(), store)

	_, err := svc.GetAlertHistory(context.Background(), "uid1", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateAlert_IncomeReceived_WithMinimum(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := dto.CreateAlertRequest{
		Type:     models.AlertTypeIncomeReceived,
		Enabled:  true,
		Delivery: []models.DeliveryMethod{models.DeliveryPush},
		Config:   models.AlertConfig{AmountMinor: 100000},
	}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- delivery validation tests ---

func TestCreateAlert_EmptyDelivery(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Delivery = []models.DeliveryMethod{}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreateAlert_InvalidDeliveryMethod(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Delivery = []models.DeliveryMethod{"telegram"}
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

// --- type validation ---

func TestCreateAlert_UnknownType(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	req := validSpendThresholdReq()
	req.Type = "budget_exceeded"
	_, err := svc.CreateAlert(context.Background(), "uid1", req)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

// --- UpdateAlert tests ---

func TestUpdateAlert_OK(t *testing.T) {
	store := newFakeAlertStore()
	svc := newAlertSvc(store)
	created, err := svc.CreateAlert(context.Background(), "uid1", validSpendThresholdReq())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	updated, err := svc.UpdateAlert(context.Background(), "uid1", created.AlertID, dto.UpdateAlertRequest{
		Enabled:  false,
		Delivery: []models.DeliveryMethod{models.DeliverySMS},
		Config: models.AlertConfig{
			AmountMinor: 75000,
			Window:      dto.Window7Day,
			Dimension:   dto.DimensionOverall,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Enabled {
		t.Error("expected Enabled=false")
	}
	if updated.Config.AmountMinor != 75000 {
		t.Errorf("expected AmountMinor=75000, got %d", updated.Config.AmountMinor)
	}
}

func TestUpdateAlert_NotFound(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	_, err := svc.UpdateAlert(context.Background(), "uid1", "nonexistent", dto.UpdateAlertRequest{
		Delivery: []models.DeliveryMethod{models.DeliveryEmail},
		Config:   models.AlertConfig{AmountMinor: 1000, Window: dto.Window30Day, Dimension: dto.DimensionOverall},
	})
	if !isNotFoundError(err) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
}

// --- DeleteAlert tests ---

func TestDeleteAlert_OK(t *testing.T) {
	store := newFakeAlertStore()
	svc := newAlertSvc(store)
	created, err := svc.CreateAlert(context.Background(), "uid1", validSpendThresholdReq())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := svc.DeleteAlert(context.Background(), "uid1", created.AlertID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.alerts[created.AlertID]; ok {
		t.Error("expected alert to be removed from store")
	}
}

func TestDeleteAlert_NotFound(t *testing.T) {
	svc := newAlertSvc(newFakeAlertStore())
	err := svc.DeleteAlert(context.Background(), "uid1", "nonexistent")
	if !isNotFoundError(err) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
}
