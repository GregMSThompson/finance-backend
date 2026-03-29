package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"
	kms "cloud.google.com/go/kms/apiv1"
	"firebase.google.com/go/v4/auth"

	plaidclient "github.com/GregMSThompson/finance-backend/internal/client/plaid"
	vertexclient "github.com/GregMSThompson/finance-backend/internal/client/vertex"
	"github.com/GregMSThompson/finance-backend/internal/config"
)

// APIBootstrap contains the runtime dependencies required by the API binary.
type APIBootstrap struct {
	Log           *slog.Logger
	Firestore     *firestore.Client
	Firebase      *auth.Client
	KMS           *kms.KeyManagementClient
	PlaidAdapter  *plaidclient.Adapter
	VertexAdapter *vertexclient.Adapter
}

// RunAPI initializes the runtime dependencies required by the API binary.
func RunAPI(cfg *config.APIConfig) (*APIBootstrap, error) {
	applicationCtx := context.Background()
	bs := &APIBootstrap{
		Log: newLogger(cfg.CommonConfig),
	}

	var err error

	bs.Firestore, err = newFirestoreClient(applicationCtx, cfg.CommonConfig)
	if err != nil {
		return bs, err
	}

	fbClients, err := initFirebase(applicationCtx, cfg.ProjectID)
	if err != nil {
		return bs, err
	}
	bs.Firebase = fbClients.Auth

	bs.KMS, err = newKMSClient(applicationCtx)
	if err != nil {
		return bs, err
	}

	bs.PlaidAdapter = newPlaidAdapter(cfg)
	bs.VertexAdapter, err = newVertexAdapter(applicationCtx, cfg, bs.Log)
	if err != nil {
		return bs, err
	}

	return bs, nil
}

// Close releases the clients owned by the API bootstrap.
func (bs *APIBootstrap) Close() {
	if bs == nil {
		return
	}

	closeVertex(bs.Log, bs.VertexAdapter)
	closeFirestore(bs.Log, bs.Firestore)
	closeKMS(bs.Log, bs.KMS)
}
