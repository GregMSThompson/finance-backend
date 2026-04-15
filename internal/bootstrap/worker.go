package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/messaging"

	"github.com/GregMSThompson/finance-backend/internal/config"
)

// WorkerBootstrap contains the runtime dependencies required by the worker binary.
type WorkerBootstrap struct {
	Log       *slog.Logger
	Firestore *firestore.Client
	Messaging *messaging.Client
}

// RunWorker initializes the runtime dependencies required by the worker binary.
func RunWorker(cfg *config.WorkerConfig) (*WorkerBootstrap, error) {
	applicationCtx := context.Background()
	bs := &WorkerBootstrap{
		Log: newLogger(cfg.CommonConfig),
	}

	var err error

	bs.Firestore, err = newFirestoreClient(applicationCtx, cfg.CommonConfig)
	if err != nil {
		return bs, err
	}

	bs.Messaging, err = initFirebaseMessaging(applicationCtx, cfg.ProjectID)
	if err != nil {
		return bs, err
	}

	return bs, nil
}

// Close releases the clients owned by the worker bootstrap.
func (bs *WorkerBootstrap) Close() {
	if bs == nil {
		return
	}

	closeFirestore(bs.Log, bs.Firestore)
}
