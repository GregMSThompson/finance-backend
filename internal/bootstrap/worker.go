package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"
	kms "cloud.google.com/go/kms/apiv1"
	"firebase.google.com/go/v4/messaging"

	plaidclient "github.com/GregMSThompson/finance-backend/internal/client/plaid"
	"github.com/GregMSThompson/finance-backend/internal/config"
)

// WorkerBootstrap contains the runtime dependencies required by the worker binary.
type WorkerBootstrap struct {
	Log          *slog.Logger
	Firestore    *firestore.Client
	Messaging    *messaging.Client
	KMS          *kms.KeyManagementClient
	PlaidAdapter *plaidclient.Adapter
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

	bs.KMS, err = newKMSClient(applicationCtx)
	if err != nil {
		return bs, err
	}

	// Worker doesn't call CreateLinkToken, so no webhook URL or OAuth redirect is needed.
	bs.PlaidAdapter = plaidclient.NewAdapter(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment, "", "")

	return bs, nil
}

// Close releases the clients owned by the worker bootstrap.
func (bs *WorkerBootstrap) Close() {
	if bs == nil {
		return
	}

	closeFirestore(bs.Log, bs.Firestore)
	closeKMS(bs.Log, bs.KMS)
}
