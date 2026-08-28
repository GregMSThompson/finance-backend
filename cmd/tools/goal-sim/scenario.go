package main

import (
	"context"
	"fmt"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// scenario is a self-contained goal simulation: the goal, a day-by-day data
// progression, and the expected state at each point.
type scenario struct {
	Name        string         `yaml:"name"`
	UserID      string         `yaml:"userId"`
	Goal        scenarioGoal   `yaml:"goal"`
	Replay      replayRange    `yaml:"replay"`
	Steps       []scenarioStep `yaml:"steps"`
	FinalExpect *finalExpect   `yaml:"finalExpect"`
}

type scenarioGoal struct {
	GoalID          string          `yaml:"goalId"`
	Name            string          `yaml:"name"`
	Type            string          `yaml:"type"`
	TargetValue     float64         `yaml:"targetValue"`
	TimeWindow      string          `yaml:"timeWindow"`
	Recurrence      string          `yaml:"recurrence"`
	CreatedAt       string          `yaml:"createdAt"`
	EndDate         string          `yaml:"endDate"`
	Filters         scenarioFilters `yaml:"filters"`
	AlertThresholds scenarioAlerts  `yaml:"alertThresholds"`
}

type scenarioFilters struct {
	PFCPrimary string `yaml:"pfcPrimary"`
	Merchant   string `yaml:"merchant"`
	AccountID  string `yaml:"accountId"`
}

type scenarioAlerts struct {
	ProgressPercent *float64 `yaml:"progressPercent"`
}

type replayRange struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type scenarioStep struct {
	Date   string       `yaml:"date"`
	Add    []scenarioTx `yaml:"add"`
	Expect *stepExpect  `yaml:"expect"`
}

type scenarioTx struct {
	Name       string  `yaml:"name"`
	Amount     float64 `yaml:"amount"`
	Currency   string  `yaml:"currency"`
	PFCPrimary string  `yaml:"pfcPrimary"`
	Merchant   string  `yaml:"merchant"`
	AccountID  string  `yaml:"accountId"`
}

// stepExpect holds optional assertions for a day. Nil fields are not asserted.
type stepExpect struct {
	CurrentValue    *float64 `yaml:"currentValue"`
	PercentComplete *float64 `yaml:"percentComplete"`
	IsOnTrack       *bool    `yaml:"isOnTrack"`
	Notified        *bool    `yaml:"notified"`
	Status          *string  `yaml:"status"`
}

type finalExpect struct {
	Status *string `yaml:"status"`
}

// buildGoal maps the scenario's goal definition to a models.Goal, seeded as an
// active goal on its stated creation date.
func (sc *scenario) buildGoal() (*models.Goal, error) {
	createdAt, err := time.Parse(dateLayout, sc.Goal.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("goal.createdAt: %w", err)
	}
	// Scenario YAML expresses money in major units for readability; convert to
	// the minor units the model stores.
	targetMinor, err := helpers.ToMinorUnits(sc.Goal.TargetValue, helpers.CurrencyUSD)
	if err != nil {
		return nil, fmt.Errorf("goal.targetValue: %w", err)
	}
	return &models.Goal{
		GoalID:           sc.Goal.GoalID,
		Type:             models.GoalType(sc.Goal.Type),
		Name:             sc.Goal.Name,
		TargetValueMinor: targetMinor,
		TimeWindow:       models.GoalTimeWindow(sc.Goal.TimeWindow),
		EndDate:          sc.Goal.EndDate,
		Recurrence:       models.GoalRecurrence(sc.Goal.Recurrence),
		Filters: models.GoalFilters{
			PFCPrimary: sc.Goal.Filters.PFCPrimary,
			Merchant:   sc.Goal.Filters.Merchant,
			AccountID:  sc.Goal.Filters.AccountID,
		},
		AlertThresholds: models.GoalAlertThresholds{
			ProgressPercent: sc.Goal.AlertThresholds.ProgressPercent,
		},
		Status:    models.GoalStatusActive,
		CreatedAt: createdAt,
	}, nil
}

// goalGetter / txUpserter are the narrow store views the sim helpers need,
// declared here because the concrete store types are unexported.
type goalGetter interface {
	Get(ctx context.Context, uid, goalID string) (*models.Goal, error)
}

type txUpserter interface {
	UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error
}
