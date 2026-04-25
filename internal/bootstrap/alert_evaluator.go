package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"

	cloudtasksclient "github.com/GregMSThompson/finance-backend/internal/client/cloudtasks"
	"github.com/GregMSThompson/finance-backend/internal/config"
)

// AlertEvaluatorBootstrap contains the runtime dependencies required by the alert evaluator job.
type AlertEvaluatorBootstrap struct {
	Log        *slog.Logger
	Firestore  *firestore.Client
	CloudTasks *cloudtasksclient.Adapter
}

// RunAlertEvaluator initializes the runtime dependencies required by the alert evaluator job.
func RunAlertEvaluator(cfg *config.AlertEvaluatorConfig) (*AlertEvaluatorBootstrap, error) {
	applicationCtx := context.Background()
	bs := &AlertEvaluatorBootstrap{
		Log: newLogger(cfg.CommonConfig),
	}

	var err error

	bs.Firestore, err = newFirestoreClient(applicationCtx, cfg.CommonConfig)
	if err != nil {
		return bs, err
	}

	bs.CloudTasks, err = cloudtasksclient.NewAdapter(
		applicationCtx,
		cfg.ProjectID,
		cfg.Region,
		cfg.CloudTasksQueue,
		cfg.WorkerAudience,
		cfg.WorkerURL,
		cfg.WorkerServiceAcct,
	)
	if err != nil {
		return bs, err
	}

	return bs, nil
}

// Close releases the clients owned by the alert evaluator bootstrap.
func (bs *AlertEvaluatorBootstrap) Close() {
	if bs == nil {
		return
	}

	closeFirestore(bs.Log, bs.Firestore)

	if bs.CloudTasks != nil {
		bs.CloudTasks.Close()
	}
}
