package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

// --- fakes ---

type fakeGoalStore struct {
	goals            map[string]*models.Goal
	getErr           error
	updateErr        error
	deleteErr        error
	deleted          []string
	lastListStatuses []models.GoalStatus
}

func newFakeGoalStore() *fakeGoalStore {
	return &fakeGoalStore{goals: map[string]*models.Goal{}}
}

func (f *fakeGoalStore) Create(_ context.Context, _ string, g *models.Goal) error {
	f.goals[g.GoalID] = g
	return nil
}

func (f *fakeGoalStore) Get(_ context.Context, _, goalID string) (*models.Goal, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	g, ok := f.goals[goalID]
	if !ok {
		return nil, errs.NewNotFoundError("goal not found")
	}
	return g, nil
}

func (f *fakeGoalStore) List(_ context.Context, _ string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	f.lastListStatuses = statuses
	out := make([]*models.Goal, 0, len(f.goals))
	for _, g := range f.goals {
		out = append(out, g)
	}
	return out, nil
}

func (f *fakeGoalStore) Update(_ context.Context, _ string, g *models.Goal) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.goals[g.GoalID] = g
	return nil
}

func (f *fakeGoalStore) Delete(_ context.Context, _, goalID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, goalID)
	delete(f.goals, goalID)
	return nil
}

type fakeGoalSnapshotStore struct {
	latest         *models.GoalSnapshot
	latestErr      error
	forGoal        []*models.GoalSnapshot
	lastListLimit  int
	deletedForGoal []string
}

func (f *fakeGoalSnapshotStore) Latest(_ context.Context, _, _ string) (*models.GoalSnapshot, error) {
	return f.latest, f.latestErr
}

func (f *fakeGoalSnapshotStore) ListForGoal(_ context.Context, _, _ string, limit int) ([]*models.GoalSnapshot, error) {
	f.lastListLimit = limit
	return f.forGoal, nil
}

func (f *fakeGoalSnapshotStore) DeleteForGoal(_ context.Context, _, goalID string) error {
	f.deletedForGoal = append(f.deletedForGoal, goalID)
	return nil
}

// --- helpers ---

func validGoalDef() dto.GoalDefinition {
	return dto.GoalDefinition{
		Type:        models.GoalTypeSpendingLimit,
		Name:        "Dining — Monthly",
		TargetValue: 220,
		TimeWindow:  models.GoalWindowMonthly,
		Recurrence:  models.GoalRecurrenceRecurring,
		Filters:     models.GoalFilters{PFCPrimary: "FOOD_AND_DRINK"},
	}
}

func seedGoal(store *fakeGoalStore) *models.Goal {
	g := &models.Goal{
		GoalID:      "g1",
		Type:        models.GoalTypeSpendingLimit,
		Name:        "Dining — Monthly",
		TargetValue: 220,
		TimeWindow:  models.GoalWindowMonthly,
		Recurrence:  models.GoalRecurrenceRecurring,
		Status:      models.GoalStatusActive,
	}
	store.goals[g.GoalID] = g
	return g
}

// --- Create ---

func TestGoalCreate_Valid(t *testing.T) {
	goals := newFakeGoalStore()
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{})

	g, err := svc.Create(context.Background(), "uid1", "session-1", validGoalDef())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.GoalID == "" {
		t.Fatal("expected GoalID to be set")
	}
	if g.Status != models.GoalStatusActive {
		t.Fatalf("expected active status, got %s", g.Status)
	}
	if g.ConversationID != "session-1" {
		t.Fatalf("expected conversationId=session-1, got %q", g.ConversationID)
	}
}

func TestGoalCreate_ZeroTarget(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{})
	def := validGoalDef()
	def.TargetValue = 0
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestGoalCreate_RecurringFixedRejected(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{})
	def := validGoalDef()
	def.TimeWindow = models.GoalWindowFixed
	def.EndDate = "2026-12-31"
	// recurrence stays recurring — invalid combo
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for recurring+fixed, got %v", err)
	}
}

func TestGoalCreate_FixedRequiresEndDate(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{})
	def := validGoalDef()
	def.TimeWindow = models.GoalWindowFixed
	def.Recurrence = models.GoalRecurrenceOneOff
	def.EndDate = "" // missing
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for fixed without endDate, got %v", err)
	}
}

func TestGoalCreate_InvalidCategory(t *testing.T) {
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, &fakeJobs{})
	def := validGoalDef()
	def.Filters.PFCPrimary = "NOT_A_CATEGORY"
	_, err := svc.Create(context.Background(), "uid1", "s", def)
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for bad category, got %v", err)
	}
}

// --- Update ---

func TestGoalUpdate_PartialMerge(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{})

	updated, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		TargetValue: helpers.Ptr(300.0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.TargetValue != 300 {
		t.Fatalf("expected target 300, got %v", updated.TargetValue)
	}
	if updated.Name != "Dining — Monthly" {
		t.Fatalf("name should be unchanged, got %q", updated.Name)
	}
}

func TestGoalUpdate_RevalidatesMergedGoal(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{})

	_, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		TargetValue: helpers.Ptr(-5.0),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for negative target, got %v", err)
	}
}

func TestGoalUpdate_PauseOK(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{})

	updated, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		Status: helpers.Ptr(models.GoalStatusPaused),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != models.GoalStatusPaused {
		t.Fatalf("expected paused, got %s", updated.Status)
	}
}

func TestGoalUpdate_CompletedRejected(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{}, &fakeJobs{})

	_, err := svc.Update(context.Background(), "uid1", "g1", dto.GoalUpdate{
		Status: helpers.Ptr(models.GoalStatusCompleted),
	})
	if !isValidationError(err) {
		t.Fatalf("expected ValidationError for manual completed, got %v", err)
	}
}

// --- Delete ---

func TestGoalDelete_SubmitsJob(t *testing.T) {
	jobs := &fakeJobs{jobID: "job-xyz"}
	svc := NewGoalService(newFakeGoalStore(), &fakeGoalSnapshotStore{}, jobs)

	got, err := svc.Delete(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "job-xyz" {
		t.Fatalf("expected jobID job-xyz, got %q", got)
	}
	if jobs.gotTyp != models.JobTypeGoalDelete {
		t.Fatalf("expected JobTypeGoalDelete, got %s", jobs.gotTyp)
	}
	var params dto.GoalDeleteParams
	if err := json.Unmarshal(jobs.gotRaw, &params); err != nil {
		t.Fatalf("params not valid json: %v", err)
	}
	if params.GoalID != "g1" {
		t.Fatalf("expected params.GoalID=g1, got %q", params.GoalID)
	}
}

func TestGoalRunDelete_Cascades(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	snaps := &fakeGoalSnapshotStore{}
	svc := NewGoalService(goals, snaps, &fakeJobs{})

	_, err := svc.RunDelete(context.Background(), "uid1", dto.GoalDeleteParams{GoalID: "g1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps.deletedForGoal) != 1 || snaps.deletedForGoal[0] != "g1" {
		t.Fatalf("expected snapshots deleted for g1, got %v", snaps.deletedForGoal)
	}
	if len(goals.deleted) != 1 || goals.deleted[0] != "g1" {
		t.Fatalf("expected goal g1 deleted, got %v", goals.deleted)
	}
}

// --- GetProgress ---

func TestGoalGetProgress_NoSnapshot(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	svc := NewGoalService(goals, &fakeGoalSnapshotStore{latest: nil}, &fakeJobs{})

	prog, err := svc.GetProgress(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog.CurrentValue != 0 || prog.AmountRemaining != 220 {
		t.Fatalf("expected zero progress, remaining=220, got current=%v remaining=%v", prog.CurrentValue, prog.AmountRemaining)
	}
	if !prog.IsOnTrack {
		t.Fatal("expected isOnTrack=true for un-evaluated goal")
	}
	if !prog.AsOf.IsZero() {
		t.Fatalf("expected zero AsOf for un-evaluated goal, got %v", prog.AsOf)
	}
}

func TestGoalGetProgress_WithSnapshot(t *testing.T) {
	goals := newFakeGoalStore()
	seedGoal(goals)
	at := time.Date(2026, time.July, 21, 2, 0, 0, 0, time.UTC)
	snaps := &fakeGoalSnapshotStore{latest: &models.GoalSnapshot{
		CurrentValue:    150,
		PercentComplete: 68.2,
		IsOnTrack:       true,
		AIInsight:       "You're on track.",
		CreatedAt:       at,
	}}
	svc := NewGoalService(goals, snaps, &fakeJobs{})

	prog, err := svc.GetProgress(context.Background(), "uid1", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog.CurrentValue != 150 || prog.AmountRemaining != 70 {
		t.Fatalf("expected current=150 remaining=70, got current=%v remaining=%v", prog.CurrentValue, prog.AmountRemaining)
	}
	if prog.PercentComplete != 68.2 || prog.AIInsight != "You're on track." {
		t.Fatalf("unexpected progress fields: %+v", prog)
	}
	if !prog.AsOf.Equal(at) {
		t.Fatalf("expected AsOf=%v, got %v", at, prog.AsOf)
	}
}
