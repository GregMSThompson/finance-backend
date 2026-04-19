package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type alertStore interface {
	Create(ctx context.Context, uid string, a *models.Alert) error
	Get(ctx context.Context, uid, alertID string) (*models.Alert, error)
	List(ctx context.Context, uid string, activeOnly bool) ([]*models.Alert, error)
	Update(ctx context.Context, uid string, a *models.Alert) error
	Delete(ctx context.Context, uid, alertID string) error
}

type alertService struct {
	store alertStore
}

func NewAlertService(store alertStore) *alertService {
	return &alertService{store: store}
}

func (s *alertService) CreateAlert(ctx context.Context, uid string, req dto.CreateAlertRequest) (*models.Alert, error) {
	if err := validateAlertType(req.Type); err != nil {
		return nil, err
	}
	if err := validateDelivery(req.Delivery); err != nil {
		return nil, err
	}
	if err := validateAlertConfig(req.Type, req.Config); err != nil {
		return nil, err
	}
	a := &models.Alert{
		AlertID:  uuid.New().String(),
		Type:     req.Type,
		Enabled:  req.Enabled,
		Delivery: req.Delivery,
		Config:   req.Config,
	}
	if err := s.store.Create(ctx, uid, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *alertService) GetAlerts(ctx context.Context, uid string) ([]*models.Alert, error) {
	return s.store.List(ctx, uid, false)
}

func (s *alertService) UpdateAlert(ctx context.Context, uid, alertID string, req dto.UpdateAlertRequest) (*models.Alert, error) {
	a, err := s.store.Get(ctx, uid, alertID)
	if err != nil {
		return nil, err
	}
	if err := validateDelivery(req.Delivery); err != nil {
		return nil, err
	}
	if err := validateAlertConfig(a.Type, req.Config); err != nil {
		return nil, err
	}
	a.Enabled = req.Enabled
	a.Delivery = req.Delivery
	a.Config = req.Config
	if err := s.store.Update(ctx, uid, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *alertService) DeleteAlert(ctx context.Context, uid, alertID string) error {
	if _, err := s.store.Get(ctx, uid, alertID); err != nil {
		return err
	}
	return s.store.Delete(ctx, uid, alertID)
}

func validateAlertType(t models.AlertType) error {
	switch t {
	case models.AlertTypeSpendThreshold, models.AlertTypeLargeTransaction,
		models.AlertTypeSubscriptionIncrease, models.AlertTypeIncomeReceived:
		return nil
	}
	return errs.NewValidationError("unknown alert type: " + string(t))
}

func validateDelivery(delivery []models.DeliveryMethod) error {
	if len(delivery) == 0 {
		return errs.NewValidationError("at least one delivery method is required")
	}
	for _, d := range delivery {
		switch d {
		case models.DeliveryEmail, models.DeliverySMS, models.DeliveryPush:
		default:
			return errs.NewValidationError(fmt.Sprintf("unknown delivery method: %q", d))
		}
	}
	return nil
}

func validateAlertConfig(t models.AlertType, cfg models.AlertConfig) error {
	switch t {
	case models.AlertTypeSpendThreshold:
		if cfg.AmountMinor <= 0 {
			return errs.NewValidationError("config.amountMinor must be greater than 0 for spend_threshold")
		}
		switch cfg.Window {
		case dto.Window7Day, dto.Window30Day, dto.Window60Day, dto.Window90Day:
		default:
			return errs.NewValidationError("config.window must be one of: 7day, 30day, 60day, 90day")
		}
		switch cfg.Dimension {
		case dto.DimensionOverall:
		case dto.DimensionCategory:
			if cfg.Category == "" {
				return errs.NewValidationError("config.category is required when dimension is \"category\"")
			}
		case dto.DimensionMerchant:
			if cfg.Merchant == "" {
				return errs.NewValidationError("config.merchant is required when dimension is \"merchant\"")
			}
		default:
			return errs.NewValidationError("config.dimension must be one of: overall, category, merchant")
		}

	case models.AlertTypeLargeTransaction:
		if cfg.AmountMinor <= 0 {
			return errs.NewValidationError("config.amountMinor must be greater than 0 for large_transaction")
		}

	case models.AlertTypeSubscriptionIncrease:
		// merchant is optional — no required fields

	case models.AlertTypeIncomeReceived:
		// amountMinor is optional — no required fields
	}
	return nil
}
