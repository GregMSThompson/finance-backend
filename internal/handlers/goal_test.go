package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

// --- stub service ---

type stubGoalService struct {
	listResp           []*models.Goal
	lastListStatuses   []models.GoalStatus
	getResp            *models.Goal
	progressResp       dto.GoalProgress
	snapshotsResp      []*models.GoalSnapshot
	lastSnapshotsLimit int
	txResp             dto.TransactionListResult
	lastTxCursor       *string
	lastTxLimit        int
	updateResp         *models.Goal
	lastUpdate         dto.GoalUpdate
	deleteJobID        string
	lastDeleteID       string
}

func (s *stubGoalService) List(_ context.Context, _ string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	s.lastListStatuses = statuses
	return s.listResp, nil
}

func (s *stubGoalService) Get(_ context.Context, _, _ string) (*models.Goal, error) {
	return s.getResp, nil
}

func (s *stubGoalService) GetProgress(_ context.Context, _, _ string) (dto.GoalProgress, error) {
	return s.progressResp, nil
}

func (s *stubGoalService) ListSnapshots(_ context.Context, _, _ string, limit int) ([]*models.GoalSnapshot, error) {
	s.lastSnapshotsLimit = limit
	return s.snapshotsResp, nil
}

func (s *stubGoalService) ListGoalTransactions(_ context.Context, _, _ string, cursor *string, limit int) (dto.TransactionListResult, error) {
	s.lastTxCursor = cursor
	s.lastTxLimit = limit
	return s.txResp, nil
}

func (s *stubGoalService) Update(_ context.Context, _, _ string, upd dto.GoalUpdate) (*models.Goal, error) {
	s.lastUpdate = upd
	return s.updateResp, nil
}

func (s *stubGoalService) Delete(_ context.Context, _, goalID string) (string, error) {
	s.lastDeleteID = goalID
	return s.deleteJobID, nil
}

func newGoalHandlers(svc goalService, resp *stubResponseHandler) *goalHandlers {
	return &goalHandlers{ResponseHandler: resp, GoalSvc: svc}
}

// --- tests ---

func TestGetGoals_DefaultStatuses(t *testing.T) {
	svc := &stubGoalService{}
	resp := &stubResponseHandler{}
	h := newGoalHandlers(svc, resp)

	req := withUID(httptest.NewRequest(http.MethodGet, "/goals", nil), "uid1")
	h.GetGoals(httptest.NewRecorder(), req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if len(svc.lastListStatuses) != 2 ||
		svc.lastListStatuses[0] != models.GoalStatusActive ||
		svc.lastListStatuses[1] != models.GoalStatusPaused {
		t.Fatalf("expected default [active paused], got %v", svc.lastListStatuses)
	}
}

func TestGetGoals_StatusFilter(t *testing.T) {
	svc := &stubGoalService{}
	h := newGoalHandlers(svc, &stubResponseHandler{})

	req := withUID(httptest.NewRequest(http.MethodGet, "/goals?status=completed&status=failed", nil), "uid1")
	h.GetGoals(httptest.NewRecorder(), req)

	if len(svc.lastListStatuses) != 2 ||
		svc.lastListStatuses[0] != models.GoalStatusCompleted ||
		svc.lastListStatuses[1] != models.GoalStatusFailed {
		t.Fatalf("expected [completed failed], got %v", svc.lastListStatuses)
	}
}

func TestGetGoals_InvalidStatus(t *testing.T) {
	resp := &stubResponseHandler{}
	h := newGoalHandlers(&stubGoalService{}, resp)

	req := withUID(httptest.NewRequest(http.MethodGet, "/goals?status=bogus", nil), "uid1")
	h.GetGoals(httptest.NewRecorder(), req)

	if !resp.handleErrorCalled {
		t.Fatal("expected error for invalid status")
	}
}

func TestGetGoalSnapshots_DefaultLimit(t *testing.T) {
	svc := &stubGoalService{}
	h := newGoalHandlers(svc, &stubResponseHandler{})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/goals/g1/snapshots", nil), "goalId", "g1"), "uid1")
	h.GetGoalSnapshots(httptest.NewRecorder(), req)

	if svc.lastSnapshotsLimit != goalSnapshotsDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", goalSnapshotsDefaultLimit, svc.lastSnapshotsLimit)
	}
}

func TestGetGoalSnapshots_LimitOverride(t *testing.T) {
	svc := &stubGoalService{}
	h := newGoalHandlers(svc, &stubResponseHandler{})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/goals/g1/snapshots?limit=10", nil), "goalId", "g1"), "uid1")
	h.GetGoalSnapshots(httptest.NewRecorder(), req)

	if svc.lastSnapshotsLimit != 10 {
		t.Fatalf("expected limit 10, got %d", svc.lastSnapshotsLimit)
	}
}

func TestPatchGoal_MapsNarrowFields(t *testing.T) {
	svc := &stubGoalService{updateResp: &models.Goal{GoalID: "g1"}}
	h := newGoalHandlers(svc, &stubResponseHandler{})

	// Body includes a substantive field (targetValue) that PatchGoalRequest
	// can't decode — it must be ignored.
	body := `{"status":"paused","alertThresholds":{"progressPercent":80},"targetValue":999}`
	req := withUID(withChiParam(httptest.NewRequest(http.MethodPatch, "/goals/g1", strings.NewReader(body)), "goalId", "g1"), "uid1")
	h.PatchGoal(httptest.NewRecorder(), req)

	if svc.lastUpdate.Status == nil || *svc.lastUpdate.Status != models.GoalStatusPaused {
		t.Fatalf("expected status paused, got %v", svc.lastUpdate.Status)
	}
	if svc.lastUpdate.AlertThresholds == nil || svc.lastUpdate.AlertThresholds.ProgressPercent == nil {
		t.Fatal("expected alertThresholds to be mapped")
	}
	if svc.lastUpdate.TargetValueMinor != nil {
		t.Fatalf("substantive field targetValue must not be settable via PATCH, got %v", *svc.lastUpdate.TargetValueMinor)
	}
}

func TestDeleteGoal_Returns202WithJob(t *testing.T) {
	svc := &stubGoalService{deleteJobID: "job-xyz"}
	resp := &stubResponseHandler{}
	h := newGoalHandlers(svc, resp)

	req := withUID(withChiParam(httptest.NewRequest(http.MethodDelete, "/goals/g1", nil), "goalId", "g1"), "uid1")
	h.DeleteGoal(httptest.NewRecorder(), req)

	if resp.writeSuccessStatus != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.writeSuccessStatus)
	}
	if svc.lastDeleteID != "g1" {
		t.Fatalf("expected delete for g1, got %q", svc.lastDeleteID)
	}
	job, ok := resp.writeSuccessData.(dto.SubmitJobResponse)
	if !ok || job.JobID != "job-xyz" {
		t.Fatalf("expected SubmitJobResponse with jobId job-xyz, got %+v", resp.writeSuccessData)
	}
}

func TestGetGoalTransactions_DefaultLimitAndCursor(t *testing.T) {
	svc := &stubGoalService{txResp: dto.TransactionListResult{
		Transactions: []models.Transaction{{TransactionID: "t1"}},
	}}
	resp := &stubResponseHandler{}
	h := newGoalHandlers(svc, resp)

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/goals/g1/transactions?cursor=abc", nil), "goalId", "g1"), "uid1")
	h.GetGoalTransactions(httptest.NewRecorder(), req)

	if !resp.writeSuccessCalled || resp.writeSuccessStatus != http.StatusOK {
		t.Fatalf("expected 200, got called=%v status=%d", resp.writeSuccessCalled, resp.writeSuccessStatus)
	}
	if svc.lastTxLimit != listTransactionsDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", listTransactionsDefaultLimit, svc.lastTxLimit)
	}
	if svc.lastTxCursor == nil || *svc.lastTxCursor != "abc" {
		t.Fatalf("expected cursor abc forwarded, got %v", svc.lastTxCursor)
	}
}

func TestGetGoalTransactions_LimitOverride(t *testing.T) {
	svc := &stubGoalService{}
	h := newGoalHandlers(svc, &stubResponseHandler{})

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/goals/g1/transactions?limit=10", nil), "goalId", "g1"), "uid1")
	h.GetGoalTransactions(httptest.NewRecorder(), req)

	if svc.lastTxLimit != 10 {
		t.Fatalf("expected limit 10, got %d", svc.lastTxLimit)
	}
	if svc.lastTxCursor != nil {
		t.Fatalf("expected nil cursor when omitted, got %v", *svc.lastTxCursor)
	}
}

func TestGetGoalTransactions_InvalidLimit(t *testing.T) {
	resp := &stubResponseHandler{}
	h := newGoalHandlers(&stubGoalService{}, resp)

	req := withUID(withChiParam(httptest.NewRequest(http.MethodGet, "/goals/g1/transactions?limit=0", nil), "goalId", "g1"), "uid1")
	h.GetGoalTransactions(httptest.NewRecorder(), req)

	if !resp.handleErrorCalled {
		t.Fatal("expected error for invalid limit")
	}
}
