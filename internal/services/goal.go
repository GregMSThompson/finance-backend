package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/taxonomy"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

type goalStore interface {
	Create(ctx context.Context, uid string, g *models.Goal) error
	Get(ctx context.Context, uid, goalID string) (*models.Goal, error)
	List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error)
	Update(ctx context.Context, uid string, g *models.Goal) error
	Delete(ctx context.Context, uid, goalID string) error
}

type goalSnapshotStore interface {
	Latest(ctx context.Context, uid, goalID string) (*models.GoalSnapshot, error)
	ListForGoal(ctx context.Context, uid, goalID string, limit int) ([]*models.GoalSnapshot, error)
	DeleteForGoal(ctx context.Context, uid, goalID string) error
}

type goalTransactionLister interface {
	ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error)
}

type goalService struct {
	goals        goalStore
	snapshots    goalSnapshotStore
	jobs         jobSubmitter
	transactions goalTransactionLister
	strategies   map[models.GoalType]goalStrategy
}

func NewGoalService(goals goalStore, snapshots goalSnapshotStore, jobs jobSubmitter, transactions goalTransactionLister, analytics goalStrategyAnalytics, accounts goalStrategyAccounts) *goalService {
	return &goalService{
		goals:        goals,
		snapshots:    snapshots,
		jobs:         jobs,
		transactions: transactions,
		strategies:   newGoalStrategies(analytics, accounts),
	}
}

// Create validates the primitive and persists a new active goal, linking it to
// the chat session that created it.
func (s *goalService) Create(ctx context.Context, uid, sessionID string, def dto.GoalDefinition) (*models.Goal, error) {
	g := &models.Goal{
		GoalID:           uuid.NewString(),
		Type:             def.Type,
		Name:             def.Name,
		TargetValueMinor: def.TargetValueMinor,
		// USD-only today; this is the single place a goal's currency is pinned.
		// Multi-currency will source it from the definition/user prefs.
		Currency:         helpers.CurrencyUSD,
		TimeWindow:       def.TimeWindow,
		EndDate:          def.EndDate,
		Recurrence:       def.Recurrence,
		Filters:          def.Filters,
		AlertThresholds:  def.AlertThresholds,
		ReductionPercent: def.ReductionPercent,
		MonthsOfExpenses: def.MonthsOfExpenses,
		Status:           models.GoalStatusActive,
		ConversationID:   sessionID,
	}
	if err := validateGoal(g); err != nil {
		return nil, err
	}
	strat, err := strategyFor(s.strategies, g.Type)
	if err != nil {
		return nil, err
	}
	// Pin the creation time before Initialize so a baseline-capturing strategy
	// resolves its comparable prior period relative to now.
	g.CreatedAt = clock.Now(ctx)
	if err := strat.Initialize(ctx, uid, g); err != nil {
		return nil, err
	}
	if err := s.goals.Create(ctx, uid, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *goalService) Get(ctx context.Context, uid, goalID string) (*models.Goal, error) {
	return s.goals.Get(ctx, uid, goalID)
}

func (s *goalService) List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	return s.goals.List(ctx, uid, statuses...)
}

// ListSnapshots returns up to limit of a goal's progress snapshots, most recent
// first — the progress-graph time series.
func (s *goalService) ListSnapshots(ctx context.Context, uid, goalID string, limit int) ([]*models.GoalSnapshot, error) {
	return s.snapshots.ListForGoal(ctx, uid, goalID, limit)
}

// Update applies a partial update, re-validating the merged goal. Status may
// only be moved between active and paused via the API — completed and failed
// are terminal outcomes set by evaluation.
func (s *goalService) Update(ctx context.Context, uid, goalID string, upd dto.GoalUpdate) (*models.Goal, error) {
	if upd.Status != nil {
		if err := validateStatusChange(*upd.Status); err != nil {
			return nil, err
		}
	}

	g, err := s.goals.Get(ctx, uid, goalID)
	if err != nil {
		return nil, err
	}

	// A reduction goal's target is derived from its baseline, never set directly.
	if g.Type == models.GoalTypeReduction && upd.TargetValueMinor != nil {
		return nil, errs.NewValidationError("a reduction goal's target is derived from reductionPercent; set reductionPercent instead")
	}

	applyGoalUpdate(g, upd)
	if err := validateGoal(g); err != nil {
		return nil, err
	}

	// Re-derive a reduction goal's frozen target whenever an input it's derived
	// from changes (percent, scope, or window). The baseline is re-measured
	// against the goal's original creation period, so the reference stays fixed
	// and only the scope/percent move.
	if g.Type == models.GoalTypeReduction && reductionBaselineChanged(upd) {
		strat, err := strategyFor(s.strategies, g.Type)
		if err != nil {
			return nil, err
		}
		if err := strat.Initialize(ctx, uid, g); err != nil {
			return nil, err
		}
	}

	if err := s.goals.Update(ctx, uid, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Delete submits a goal-delete job and returns its ID. The actual cascade runs
// asynchronously on the worker via RunDelete.
func (s *goalService) Delete(ctx context.Context, uid, goalID string) (string, error) {
	params, err := json.Marshal(dto.GoalDeleteParams{GoalID: goalID})
	if err != nil {
		return "", err
	}
	return s.jobs.Submit(ctx, uid, models.JobTypeGoalDelete, params)
}

// RunDelete cascades the goal deletion: snapshots first, then the goal doc.
// Each step is idempotent so a retried job picks up where it left off.
func (s *goalService) RunDelete(ctx context.Context, uid string, params dto.GoalDeleteParams) (dto.GoalDeleteResult, error) {
	if err := s.snapshots.DeleteForGoal(ctx, uid, params.GoalID); err != nil {
		return dto.GoalDeleteResult{}, err
	}
	if err := s.goals.Delete(ctx, uid, params.GoalID); err != nil {
		return dto.GoalDeleteResult{}, err
	}
	return dto.GoalDeleteResult{GoalID: params.GoalID}, nil
}

// GetProgress returns the goal's latest evaluated progress. A goal that has not
// been evaluated yet returns zeroed progress with a zero AsOf — the client's
// "not measured yet" signal.
func (s *goalService) GetProgress(ctx context.Context, uid, goalID string) (dto.GoalProgress, error) {
	g, err := s.goals.Get(ctx, uid, goalID)
	if err != nil {
		return dto.GoalProgress{}, err
	}

	prog := dto.GoalProgress{
		GoalID:           g.GoalID,
		Name:             g.Name,
		Status:           g.Status,
		TargetValueMinor: g.TargetValueMinor,
		Currency:         g.Currency,
	}

	snap, err := s.snapshots.Latest(ctx, uid, goalID)
	if err != nil {
		return dto.GoalProgress{}, err
	}
	if snap == nil {
		prog.AmountRemainingMinor = g.TargetValueMinor
		prog.IsOnTrack = true
		return prog, nil
	}

	prog.CurrentValueMinor = snap.CurrentValueMinor
	prog.AmountRemainingMinor = g.TargetValueMinor - snap.CurrentValueMinor
	prog.PercentComplete = snap.PercentComplete
	prog.IsOnTrack = snap.IsOnTrack
	prog.AIInsight = snap.AIInsight
	prog.AsOf = snap.CreatedAt
	return prog, nil
}

// ListGoalTransactions returns the transactions counting toward a goal in its
// current measurement window, paginated with the same cursor+limit contract as
// GET /transactions. The window and the category/merchant/account scope are
// derived from the goal server-side, so the list can't drift from the goal's
// definition. The window is computed at request time (up to today), so this is
// a live view — it may run slightly ahead of the last nightly snapshot.
func (s *goalService) ListGoalTransactions(ctx context.Context, uid, goalID string, cursor *string, limit int) (dto.TransactionListResult, error) {
	g, err := s.goals.Get(ctx, uid, goalID)
	if err != nil {
		return dto.TransactionListResult{}, err
	}

	now := clock.Now(ctx)
	start, end, err := g.ResolveWindow(now)
	if err != nil {
		return dto.TransactionListResult{}, err
	}
	// Cap at the earlier of today and the window end — a fixed window can close
	// before today.
	queryTo := helpers.DateOf(now)
	if queryTo.After(end) {
		queryTo = end
	}

	args := dto.TransactionListArgs{
		Pending:  helpers.Ptr(false),
		DateFrom: helpers.Ptr(helpers.FormatDate(start)),
		DateTo:   helpers.Ptr(helpers.FormatDate(queryTo)),
		OrderBy:  "date",
		Desc:     true,
		Cursor:   cursor,
		Limit:    limit,
	}
	if g.Filters.PFCPrimary != "" {
		args.PFCPrimaries = []string{g.Filters.PFCPrimary}
	}
	if g.Filters.Merchant != "" {
		args.Merchant = helpers.Ptr(g.Filters.Merchant)
	}
	if g.Filters.AccountID != "" {
		args.AccountID = helpers.Ptr(g.Filters.AccountID)
	}

	return s.transactions.ListTransactions(ctx, uid, args)
}

func applyGoalUpdate(g *models.Goal, upd dto.GoalUpdate) {
	if upd.Name != nil {
		g.Name = *upd.Name
	}
	if upd.TargetValueMinor != nil {
		g.TargetValueMinor = *upd.TargetValueMinor
	}
	if upd.TimeWindow != nil {
		g.TimeWindow = *upd.TimeWindow
	}
	if upd.EndDate != nil {
		g.EndDate = *upd.EndDate
	}
	if upd.Recurrence != nil {
		g.Recurrence = *upd.Recurrence
	}
	if upd.Filters != nil {
		g.Filters = *upd.Filters
	}
	if upd.AlertThresholds != nil {
		g.AlertThresholds = *upd.AlertThresholds
	}
	if upd.Status != nil {
		g.Status = *upd.Status
	}
	if upd.ReductionPercent != nil {
		g.ReductionPercent = upd.ReductionPercent
	}
}

// reductionBaselineChanged reports whether an update touches an input a reduction
// goal derives its target from — its percent, filter scope, or window — and so
// requires the target to be re-derived from a freshly measured baseline.
func reductionBaselineChanged(upd dto.GoalUpdate) bool {
	return upd.ReductionPercent != nil || upd.Filters != nil || upd.TimeWindow != nil || upd.EndDate != nil
}

// validateStatusChange guards the statuses a caller may set. completed and
// failed are set by the evaluator, not manually.
func validateStatusChange(status models.GoalStatus) error {
	switch status {
	case models.GoalStatusActive, models.GoalStatusPaused:
		return nil
	case models.GoalStatusCompleted, models.GoalStatusFailed:
		return errs.NewValidationError("completed and failed statuses are set by evaluation, not manually")
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid status: %s", status))
	}
}

// validateGoal enforces the goal primitive rules. Target validation is
// per-type: a spending limit carries a literal target, while a reduction goal
// carries a percent and derives its target from the baseline at creation (so
// its target is still zero at validation time).
func validateGoal(g *models.Goal) error {
	switch g.Type {
	case models.GoalTypeSpendingLimit:
		if g.TargetValueMinor <= 0 {
			return errs.NewValidationError("targetValue must be greater than 0")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
	case models.GoalTypeReduction:
		if g.ReductionPercent == nil {
			return errs.NewValidationError("reductionPercent is required for a reduction goal")
		}
		if *g.ReductionPercent <= 0 || *g.ReductionPercent >= 100 {
			return errs.NewValidationError("reductionPercent must be between 0 and 100")
		}
	case models.GoalTypeNetSavings, models.GoalTypeIncomeTarget:
		if g.TargetValueMinor <= 0 {
			return errs.NewValidationError("targetValue must be greater than 0")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
		// These are whole-finances figures measured across all accounts, so they
		// take no filters — scoping to an account is a separate goal type.
		if g.Filters != (models.GoalFilters{}) {
			return errs.NewValidationError("net savings and income goals take no filters")
		}
	case models.GoalTypeSavingsTarget:
		if g.TargetValueMinor <= 0 {
			return errs.NewValidationError("targetValue must be greater than 0")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
		// A savings target measures growth against a fixed starting balance, so a
		// recurring reset has no meaning — the baseline would move each period.
		// Per-period saving is the savings_contributions goal instead.
		if g.Recurrence == models.GoalRecurrenceRecurring {
			return errs.NewValidationError("a savings target must be one-off")
		}
		// It tracks an account balance, not spend, so category and merchant filters
		// don't apply; only an optional accountId scope is allowed.
		if g.Filters.PFCPrimary != "" || g.Filters.Merchant != "" {
			return errs.NewValidationError("a savings target can only be scoped by accountId, not category or merchant")
		}
	case models.GoalTypeSavingsContributions:
		if g.TargetValueMinor <= 0 {
			return errs.NewValidationError("targetValue must be greater than 0")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
		// Contributions are transfers into a specific destination account. Without a
		// scope, gross transfers-in across all accounts would count internal
		// account-to-account moves as contributions, so the account is required.
		if g.Filters.AccountID == "" {
			return errs.NewValidationError("a savings contributions goal must be scoped to an accountId")
		}
		// It counts transfers into that account, not spend, so category and merchant
		// filters don't apply.
		if g.Filters.PFCPrimary != "" || g.Filters.Merchant != "" {
			return errs.NewValidationError("a savings contributions goal can only be scoped by accountId, not category or merchant")
		}
	case models.GoalTypePayDown:
		if g.TargetValueMinor <= 0 {
			return errs.NewValidationError("targetValue must be greater than 0")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
		// Measures reduction against a fixed starting balance, so a recurring reset
		// would move the baseline — one-off only, same as a savings target.
		if g.Recurrence == models.GoalRecurrenceRecurring {
			return errs.NewValidationError("a pay down goal must be one-off")
		}
		// The account is required: summing balances across all accounts would net
		// asset balances against the debt, which isn't a meaningful pay-down figure.
		if g.Filters.AccountID == "" {
			return errs.NewValidationError("a pay down goal must be scoped to an accountId")
		}
		// It tracks a debt balance, not spend, so category and merchant filters don't
		// apply.
		if g.Filters.PFCPrimary != "" || g.Filters.Merchant != "" {
			return errs.NewValidationError("a pay down goal can only be scoped by accountId, not category or merchant")
		}
	case models.GoalTypeEmergencyFund:
		if g.MonthsOfExpenses == nil || *g.MonthsOfExpenses <= 0 {
			return errs.NewValidationError("monthsOfExpenses must be greater than 0 for an emergency fund goal")
		}
		if g.ReductionPercent != nil {
			return errs.NewValidationError("reductionPercent applies only to reduction goals")
		}
		// The target is derived from spend at creation, so callers don't set one.
		if g.TargetValueMinor != 0 {
			return errs.NewValidationError("targetValue is derived for an emergency fund goal; provide monthsOfExpenses instead")
		}
		// Open-ended, event-driven completion — it runs until the target is reached,
		// so it must use the until_reached window (and therefore one-off).
		if g.TimeWindow != models.GoalWindowUntilReached {
			return errs.NewValidationError("an emergency fund goal must use the until_reached window")
		}
		// The balance is read from the account holding the fund, so it's required.
		if g.Filters.AccountID == "" {
			return errs.NewValidationError("an emergency fund goal must be scoped to an accountId")
		}
		// It tracks an account balance, not spend, so category and merchant filters
		// don't apply.
		if g.Filters.PFCPrimary != "" || g.Filters.Merchant != "" {
			return errs.NewValidationError("an emergency fund goal can only be scoped by accountId, not category or merchant")
		}
	default:
		return errs.NewValidationError(fmt.Sprintf("unsupported goal type: %s", g.Type))
	}
	if strings.TrimSpace(g.Name) == "" {
		return errs.NewValidationError("name is required")
	}

	switch g.Recurrence {
	case models.GoalRecurrenceRecurring:
		if g.TimeWindow != models.GoalWindowWeekly && g.TimeWindow != models.GoalWindowMonthly {
			return errs.NewValidationError("recurring goals must use a weekly or monthly window")
		}
	case models.GoalRecurrenceOneOff:
		// any window is valid for a one-off
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid recurrence: %s", g.Recurrence))
	}

	switch g.TimeWindow {
	case models.GoalWindowFixed:
		if g.EndDate == "" {
			return errs.NewValidationError("a fixed window requires an endDate")
		}
		if !helpers.IsValidDate(g.EndDate) {
			return errs.NewValidationError(fmt.Sprintf("invalid endDate: %s", g.EndDate))
		}
	case models.GoalWindowWeekly, models.GoalWindowMonthly:
		if g.EndDate != "" {
			return errs.NewValidationError("weekly and monthly windows must not set an endDate")
		}
	case models.GoalWindowUntilReached:
		// Open-ended: no deadline, so no endDate. It runs once until reached.
		if g.EndDate != "" {
			return errs.NewValidationError("an until_reached window must not set an endDate")
		}
		if g.Recurrence != models.GoalRecurrenceOneOff {
			return errs.NewValidationError("an until_reached window must be one-off")
		}
	default:
		return errs.NewValidationError(fmt.Sprintf("invalid timeWindow: %s", g.TimeWindow))
	}

	if g.Filters.PFCPrimary != "" && !taxonomy.IsPFCPrimaryAllowed(g.Filters.PFCPrimary) {
		return errs.NewValidationError(fmt.Sprintf("invalid category: %s", g.Filters.PFCPrimary))
	}

	// Spend goals (spending_limit, reduction) measure expenditure, so a filter on
	// a non-spend category — income or a transfer — would scope them to the wrong
	// thing: GetSpendTotal treats an explicit category filter as "give me exactly
	// this category", so it would report the raw income/transfer sum instead of
	// spend. Reject it. The at-least types reject all filters above.
	if (g.Type == models.GoalTypeSpendingLimit || g.Type == models.GoalTypeReduction) &&
		g.Filters.PFCPrimary != "" && taxonomy.IsNonSpendCategory(g.Filters.PFCPrimary) {
		return errs.NewValidationError(fmt.Sprintf("a spending goal can't be scoped to the non-spend category %s", g.Filters.PFCPrimary))
	}

	if p := g.AlertThresholds.ProgressPercent; p != nil && (*p <= 0 || *p > 100) {
		return errs.NewValidationError("progressPercent must be between 0 and 100")
	}

	return nil
}
