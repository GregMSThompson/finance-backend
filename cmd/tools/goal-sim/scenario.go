package main

import (
	"context"
	"fmt"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// scenario is a self-contained goal simulation: the goal, an optional baseline
// period (for goals that derive a target from prior spend), a day-by-day data
// progression, and the expected state at each point.
type scenario struct {
	Name     string       `yaml:"name"`
	UserID   string       `yaml:"userId"`
	Goal     scenarioGoal `yaml:"goal"`
	Baseline []scenarioTx `yaml:"baseline"`
	// Accounts seeds starting balances for balance-family goals (e.g. savings
	// target). They're written before the goal is created, so the strategy's
	// Initialize captures them as the baseline. Empty for flow-based scenarios.
	Accounts    []scenarioAccount `yaml:"accounts"`
	Replay      replayRange       `yaml:"replay"`
	Steps       []scenarioStep    `yaml:"steps"`
	FinalExpect *finalExpect      `yaml:"finalExpect"`
}

// scenarioAccount is an account's starting balance (major units) for balance
// replay. The balance is a point-in-time level, unlike transactions which
// accumulate over the window.
type scenarioAccount struct {
	AccountID string  `yaml:"accountId"`
	Balance   float64 `yaml:"balance"`
}

type scenarioGoal struct {
	GoalID           string          `yaml:"goalId"`
	Name             string          `yaml:"name"`
	Type             string          `yaml:"type"`
	TargetValue      float64         `yaml:"targetValue"`
	TargetCount      int64           `yaml:"targetCount"`
	ReductionPercent *float64        `yaml:"reductionPercent"`
	MonthsOfExpenses *float64        `yaml:"monthsOfExpenses"`
	TimeWindow       string          `yaml:"timeWindow"`
	Recurrence       string          `yaml:"recurrence"`
	CreatedAt        string          `yaml:"createdAt"`
	EndDate          string          `yaml:"endDate"`
	Filters          scenarioFilters `yaml:"filters"`
	AlertThresholds  scenarioAlerts  `yaml:"alertThresholds"`
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
	Date string       `yaml:"date"`
	Add  []scenarioTx `yaml:"add"`
	// SetBalances updates account balances (accountId → absolute balance in major
	// units) before this day's evaluation, replaying how a balance-family goal's
	// measurement moves. Only the listed accounts change; others keep their prior
	// balance. Empty for flow-based scenarios.
	SetBalances map[string]float64 `yaml:"setBalances"`
	Expect      *stepExpect        `yaml:"expect"`
}

type scenarioTx struct {
	Name       string  `yaml:"name"`
	Amount     float64 `yaml:"amount"`
	Currency   string  `yaml:"currency"`
	PFCPrimary string  `yaml:"pfcPrimary"`
	Merchant   string  `yaml:"merchant"`
	AccountID  string  `yaml:"accountId"`
	// Date is only used for baseline transactions, which aren't tied to a replay
	// step; step transactions take their date from the step instead.
	Date string `yaml:"date"`
}

// stepExpect holds optional assertions for a day. Nil fields are not asserted.
// CurrentValue asserts a money goal's measurement (major units); CurrentCount
// asserts a count goal's (frequency_limit). A scenario sets whichever matches its
// goal's unit.
type stepExpect struct {
	CurrentValue    *float64 `yaml:"currentValue"`
	CurrentCount    *int64   `yaml:"currentCount"`
	PercentComplete *float64 `yaml:"percentComplete"`
	IsOnTrack       *bool    `yaml:"isOnTrack"`
	Notified        *bool    `yaml:"notified"`
	Status          *string  `yaml:"status"`
}

type finalExpect struct {
	Status *string `yaml:"status"`
}

// buildGoalDefinition maps the scenario's goal to the definition the goal
// service creates from. The service pins the creation date (from the caller's
// clock) and runs the strategy's Initialize, so a reduction goal derives its
// target from the seeded baseline rather than the YAML.
func (sc *scenario) buildGoalDefinition() (dto.GoalDefinition, error) {
	def := dto.GoalDefinition{
		Type:             models.GoalType(sc.Goal.Type),
		Name:             sc.Goal.Name,
		TargetCount:      sc.Goal.TargetCount,
		ReductionPercent: sc.Goal.ReductionPercent,
		MonthsOfExpenses: sc.Goal.MonthsOfExpenses,
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
	}
	// Most types carry an explicit money target (converted below). Two groups don't:
	// reduction and emergency_fund derive their target at creation (from a prior-period
	// baseline / average spend); frequency_limit's target is a count (TargetCount),
	// not money. Only the money-target types read TargetValue. Scenario YAML uses major
	// units for readability, so convert to the minor units the model stores.
	usesMoneyTarget := def.Type != models.GoalTypeReduction &&
		def.Type != models.GoalTypeEmergencyFund &&
		def.Type != models.GoalTypeFrequencyLimit
	if usesMoneyTarget {
		targetMinor, err := helpers.ToMinorUnits(sc.Goal.TargetValue, helpers.CurrencyUSD)
		if err != nil {
			return dto.GoalDefinition{}, fmt.Errorf("goal.targetValue: %w", err)
		}
		def.TargetValueMinor = targetMinor
	}
	return def, nil
}

// goalGetter / txUpserter are the narrow store views the sim helpers need,
// declared here because the concrete store types are unexported.
type goalGetter interface {
	Get(ctx context.Context, uid, goalID string) (*models.Goal, error)
}

type txUpserter interface {
	UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error
}

type accountUpserter interface {
	UpsertBatch(ctx context.Context, uid, bankID string, accounts []models.Account) error
}
