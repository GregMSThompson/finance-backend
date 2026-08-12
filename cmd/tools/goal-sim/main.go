// goal-sim replays the goal evaluator across a series of virtual days against a
// dev Firestore, seeding transactions and asserting the resulting snapshot
// series. Each scenario file bundles the goal, a day-by-day data progression,
// and the expected state at each point.
//
// Usage:
//
//	go run ./cmd/tools/goal-sim --project=<gcp-project> --file=testdata/goals/<scenario>.yaml
//
// The scenario runs against a dedicated sim user (userId in the file), whose
// goals/snapshots/notifications/transactions are wiped and reseeded each run, so
// it never touches real user data. No real push notifications are sent.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"gopkg.in/yaml.v3"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

const dateLayout = "2006-01-02"

// percentTolerance is the allowed difference when asserting float fields.
const percentTolerance = 0.1

func main() {
	project := flag.String("project", os.Getenv("PROJECTID"), "GCP project ID (required)")
	filePath := flag.String("file", "", "Path to scenario YAML file (required)")
	flag.Parse()

	if *project == "" || *filePath == "" {
		flag.Usage()
		log.Fatal("--project and --file are required")
	}

	raw, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("read scenario: %v", err)
	}
	var sc scenario
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		log.Fatalf("parse scenario: %v", err)
	}

	// Attach an error-only logger so the evaluator's info logging doesn't clutter
	// the simulation's day-by-day output; only real problems surface.
	ctx := logger.ToContext(context.Background(), errorLogger())
	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		log.Fatalf("firestore client: %v", err)
	}
	defer client.Close()

	if err := run(ctx, client, &sc); err != nil {
		fatalWithCause(err)
	}
}

// errorLogger builds a text logger that only emits at error level.
func errorLogger() *slog.Logger {
	return logger.New("error", func(level slog.Level) slog.Handler {
		return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	})
}

// fatalWithCause prints the error and every wrapped cause, then exits non-zero.
// The errs.* types return only their message from Error(), so the underlying
// Firestore error — e.g. a FAILED_PRECONDITION carrying a create-index URL —
// is otherwise swallowed. Walking Unwrap surfaces it.
func fatalWithCause(err error) {
	log.Printf("scenario failed: %v", err)
	for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
		log.Printf("  caused by: %v", e)
	}
	os.Exit(1)
}

func run(ctx context.Context, client *firestore.Client, sc *scenario) error {
	goalStore := store.NewGoalStore(client)
	snapshotStore := store.NewGoalSnapshotStore(client)
	notificationStore := store.NewNotificationStore(client)
	transactionStore := store.NewTransactionStore(client)
	analyticsSvc := services.NewAnalyticsService(transactionStore)

	// Start from a clean sim user so each run is deterministic.
	if err := resetUser(ctx, client, sc.UserID); err != nil {
		return fmt.Errorf("reset sim user: %w", err)
	}

	goal, err := sc.buildGoal()
	if err != nil {
		return err
	}
	if err := goalStore.Create(ctx, sc.UserID, goal); err != nil {
		return fmt.Errorf("seed goal: %w", err)
	}

	from, err := time.Parse(dateLayout, sc.Replay.From)
	if err != nil {
		return fmt.Errorf("replay.from: %w", err)
	}
	to, err := time.Parse(dateLayout, sc.Replay.To)
	if err != nil {
		return fmt.Errorf("replay.to: %w", err)
	}

	// Evaluate only the sim user, with a virtual clock and no real deliveries.
	var clock time.Time
	evalSvc := services.NewGoalEvaluatorService(
		singleUserStore{uid: sc.UserID},
		goalStore, snapshotStore, analyticsSvc, notificationStore, noopTasks{},
		services.WithClock(func() time.Time { return clock }),
	)

	steps := indexSteps(sc.Steps)
	failures := 0

	fmt.Printf("scenario: %s (user %s, goal %s)\n", sc.Name, sc.UserID, goal.GoalID)
	fmt.Printf("%-12s %10s %8s %8s %8s\n", "date", "spent", "pct", "onTrack", "notified")

	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format(dateLayout)
		clock = time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)

		if step, ok := steps[key]; ok && len(step.Add) > 0 {
			if err := seedTransactions(ctx, transactionStore, sc.UserID, key, step.Add); err != nil {
				return fmt.Errorf("seed transactions on %s: %w", key, err)
			}
		}

		if err := evalSvc.Run(ctx); err != nil {
			return fmt.Errorf("evaluator run on %s: %w", key, err)
		}

		snap, err := snapshotStore.Latest(ctx, sc.UserID, goal.GoalID)
		if err != nil {
			return fmt.Errorf("read snapshot on %s: %w", key, err)
		}
		// A completed/paused goal isn't evaluated, so Latest returns a stale prior
		// snapshot. Detect that by comparing its date to today.
		evaluated := snap != nil && snap.CreatedAt.Format(dateLayout) == key
		printDay(key, snap, evaluated)

		if step, ok := steps[key]; ok && step.Expect != nil {
			failures += assertStep(ctx, goalStore, sc.UserID, goal.GoalID, key, snap, evaluated, step.Expect)
		}
	}

	if sc.FinalExpect != nil && sc.FinalExpect.Status != nil {
		g, err := goalStore.Get(ctx, sc.UserID, goal.GoalID)
		if err != nil {
			return fmt.Errorf("read final goal: %w", err)
		}
		if string(g.Status) != *sc.FinalExpect.Status {
			failures++
			fmt.Printf("  FAIL final: status = %q, want %q\n", g.Status, *sc.FinalExpect.Status)
		} else {
			fmt.Printf("  ok final: status = %q\n", g.Status)
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d assertion(s) failed", failures)
	}
	fmt.Println("all assertions passed")
	return nil
}

func printDay(date string, snap *models.GoalSnapshot, evaluated bool) {
	if !evaluated {
		fmt.Printf("%-12s %10s %8s %8s %8s  (not evaluated)\n", date, "—", "—", "—", "—")
		return
	}
	fmt.Printf("%-12s %10.2f %7.1f%% %8v %8v\n", date, snap.CurrentValue, snap.PercentComplete, snap.IsOnTrack, snap.NotificationSent)
}

// assertStep checks a step's expectations against the day's snapshot and (for
// status) the goal doc, returning the number of failed assertions.
func assertStep(ctx context.Context, goals goalGetter, uid, goalID, date string, snap *models.GoalSnapshot, evaluated bool, exp *stepExpect) int {
	failures := 0
	fail := func(format string, args ...any) {
		failures++
		fmt.Printf("  FAIL %s: %s\n", date, fmt.Sprintf(format, args...))
	}

	// Snapshot-field assertions require the goal to have been evaluated today; a
	// stale prior snapshot would assert the wrong day's values. Status comes from
	// the goal doc, so it's checked regardless (e.g. asserting a terminal status
	// on a day the goal is no longer evaluated).
	wantsSnapshotFields := exp.CurrentValue != nil || exp.PercentComplete != nil || exp.IsOnTrack != nil || exp.Notified != nil
	switch {
	case wantsSnapshotFields && !evaluated:
		fail("expected snapshot fields but the goal was not evaluated this day (terminal or paused?)")
	case evaluated:
		if exp.CurrentValue != nil && !approxEqual(snap.CurrentValue, *exp.CurrentValue) {
			fail("currentValue = %.2f, want %.2f", snap.CurrentValue, *exp.CurrentValue)
		}
		if exp.PercentComplete != nil && !approxEqual(snap.PercentComplete, *exp.PercentComplete) {
			fail("percentComplete = %.2f, want %.2f", snap.PercentComplete, *exp.PercentComplete)
		}
		if exp.IsOnTrack != nil && snap.IsOnTrack != *exp.IsOnTrack {
			fail("isOnTrack = %v, want %v", snap.IsOnTrack, *exp.IsOnTrack)
		}
		if exp.Notified != nil && snap.NotificationSent != *exp.Notified {
			fail("notified = %v, want %v", snap.NotificationSent, *exp.Notified)
		}
	}

	if exp.Status != nil {
		g, err := goals.Get(ctx, uid, goalID)
		if err != nil {
			fail("read goal for status assertion: %v", err)
		} else if string(g.Status) != *exp.Status {
			fail("status = %q, want %q", g.Status, *exp.Status)
		}
	}
	return failures
}

func seedTransactions(ctx context.Context, txs txUpserter, uid, date string, adds []scenarioTx) error {
	batch := make([]models.Transaction, 0, len(adds))
	for _, a := range adds {
		currency := a.Currency
		if currency == "" {
			currency = "USD"
		}
		batch = append(batch, models.Transaction{
			TransactionID: uuid.NewString(),
			BankID:        "sim-bank",
			AccountID:     a.AccountID,
			Name:          a.Name,
			Amount:        a.Amount,
			Currency:      currency,
			Pending:       false,
			Date:          date,
			PFCPrimary:    a.PFCPrimary,
		})
	}
	return txs.UpsertBatch(ctx, uid, batch)
}

// resetUser clears the sim user's goal-related subcollections so a rerun starts
// from a blank slate.
func resetUser(ctx context.Context, client *firestore.Client, uid string) error {
	for _, coll := range []string{"transactions", "goals", "goalSnapshots", "notifications"} {
		if err := deleteCollection(ctx, client, client.Collection("users").Doc(uid).Collection(coll)); err != nil {
			return err
		}
	}
	return nil
}

func deleteCollection(ctx context.Context, client *firestore.Client, coll *firestore.CollectionRef) error {
	bw := client.BulkWriter(ctx)
	iter := coll.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bw.End()
			return err
		}
		if _, err := bw.Delete(doc.Ref); err != nil {
			bw.End()
			return err
		}
	}
	bw.End()
	return nil
}

func indexSteps(steps []scenarioStep) map[string]scenarioStep {
	out := make(map[string]scenarioStep, len(steps))
	for _, s := range steps {
		out[s.Date] = s
	}
	return out
}

func approxEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= percentTolerance
}

// singleUserStore yields only the sim user, so a run never evaluates or mutates
// any other user's goals.
type singleUserStore struct {
	uid string
}

func (s singleUserStore) List(_ context.Context) ([]*models.User, error) {
	return []*models.User{{UID: s.uid}}, nil
}

// noopTasks swallows delivery enqueues so a simulation sends no real pushes.
type noopTasks struct{}

func (noopTasks) EnqueueNotificationDelivery(_ context.Context, _ dto.DeliverNotificationRequest) error {
	return nil
}
