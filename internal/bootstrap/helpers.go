package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"
	kms "cloud.google.com/go/kms/apiv1"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"

	plaidclient "github.com/GregMSThompson/finance-backend/internal/client/plaid"
	vertexclient "github.com/GregMSThompson/finance-backend/internal/client/vertex"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type firebaseClients struct {
	Auth      *auth.Client
	Messaging *messaging.Client
}

func newLogger(cfg config.CommonConfig) *slog.Logger {
	return logger.New(cfg.LogLevel, logger.NewCloudRunHandler)
}

func newFirestoreClient(ctx context.Context, cfg config.CommonConfig) (*firestore.Client, error) {
	return firestore.NewClient(ctx, cfg.ProjectID)
}

func initFirebase(ctx context.Context, projectID string) (*firebaseClients, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return &firebaseClients{
		Auth:      authClient,
		Messaging: msgClient,
	}, nil
}

func newKMSClient(ctx context.Context) (*kms.KeyManagementClient, error) {
	return kms.NewKeyManagementClient(ctx)
}

func newPlaidAdapter(cfg *config.APIConfig) *plaidclient.Adapter {
	return plaidclient.NewAdapter(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment)
}

func newVertexAdapter(ctx context.Context, cfg *config.APIConfig, log *slog.Logger) (*vertexclient.Adapter, error) {
	return vertexclient.NewAdapter(ctx, log, cfg.ProjectID, cfg.Region, cfg.VertexModel)
}

func closeFirestore(log *slog.Logger, client *firestore.Client) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil && log != nil {
		log.Error("firestore close failed", "error", err)
	}
}

func closeKMS(log *slog.Logger, client *kms.KeyManagementClient) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil && log != nil {
		log.Error("kms close failed", "error", err)
	}
}

func closeVertex(log *slog.Logger, adapter *vertexclient.Adapter) {
	if adapter == nil {
		return
	}
	if err := adapter.Close(); err != nil && log != nil {
		log.Error("vertex close failed", "error", err)
	}
}
