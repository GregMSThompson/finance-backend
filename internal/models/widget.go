package models

import "time"

// Widget represents a user's dashboard widget configuration stored in Firestore.
type Widget struct {
	WidgetID      string       `firestore:"widgetId" json:"widgetId"`
	Type          string       `firestore:"type" json:"type"`
	Visualization string       `firestore:"visualization" json:"visualization"`
	Position      int          `firestore:"position" json:"position"`
	Config        WidgetConfig `firestore:"config" json:"config"`
	CreatedAt     time.Time    `firestore:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time    `firestore:"updatedAt" json:"updatedAt"`
}

// WidgetConfig holds all possible configuration fields for any widget type.
// Not all fields are valid for all types; the service layer enforces per-type rules.
type WidgetConfig struct {
	DateRange *DateRangeConfig `firestore:"dateRange,omitempty" json:"dateRange,omitempty"`
	Window    string           `firestore:"window,omitempty" json:"window,omitempty"`    // "7day","30day","60day","90day"
	Preset    string           `firestore:"preset,omitempty" json:"preset,omitempty"`    // period comparison preset
	Dimension string           `firestore:"dimension,omitempty" json:"dimension,omitempty"` // "category","merchant","overall"
	Limit     int              `firestore:"limit,omitempty" json:"limit,omitempty"`
	Category  string           `firestore:"category,omitempty" json:"category,omitempty"` // optional PFC primary filter
	BankID    string           `firestore:"bankId,omitempty" json:"bankId,omitempty"`
}

// DateRangeConfig represents either a named preset or an explicit custom range.
type DateRangeConfig struct {
	Preset    string `firestore:"preset,omitempty" json:"preset,omitempty"`
	StartDate string `firestore:"startDate,omitempty" json:"startDate,omitempty"`
	EndDate   string `firestore:"endDate,omitempty" json:"endDate,omitempty"`
}
