package dto

import "github.com/GregMSThompson/finance-backend/internal/models"

// CreateAlertRequest is the request body for creating a new alert.
type CreateAlertRequest struct {
	Type     models.AlertType        `json:"type"`
	Enabled  bool                    `json:"enabled"`
	Delivery []models.DeliveryMethod `json:"delivery"`
	Config   models.AlertConfig      `json:"config"`
}

// UpdateAlertRequest is the request body for updating an existing alert.
type UpdateAlertRequest struct {
	Enabled  bool                    `json:"enabled"`
	Delivery []models.DeliveryMethod `json:"delivery"`
	Config   models.AlertConfig      `json:"config"`
}
