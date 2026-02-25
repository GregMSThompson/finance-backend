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
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type Bootstrap struct {
	Log           *slog.Logger
	Firestore     *firestore.Client
	Firebase      *auth.Client
	KMS           *kms.KeyManagementClient
	PlaidAdapter  *plaidclient.Adapter
	VertexAdapter *vertexclient.Adapter
}

func Run(cfg *config.Config) (*Bootstrap, error) {
	var err error
	applicationCtx := context.Background()
	bs := new(Bootstrap)

	// Logging is initialized early so downstream setup can record failures.
	bs.Log = logger.New(cfg.LogLevel, logger.NewCloudRunHandler)

	// Core cloud clients for storage/auth/crypto services.
	bs.Firestore, err = firestore.NewClient(applicationCtx, cfg.ProjectID)
	if err != nil {
		return bs, err
	}
	bs.Firebase, err = InitFirebase(applicationCtx)
	if err != nil {
		return bs, err
	}
	bs.KMS, err = kms.NewKeyManagementClient(applicationCtx)
	if err != nil {
		return bs, err
	}

	// Adapters wrap external APIs for the service layer.
	bs.PlaidAdapter = plaidclient.NewAdapter(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment)
	bs.VertexAdapter, err = vertexclient.NewAdapter(applicationCtx, bs.Log, cfg.ProjectID, cfg.Region, cfg.VertexModel)
	if err != nil {
		return bs, err
	}

	return bs, nil
}

func (bs *Bootstrap) Close() {
	if bs == nil {
		return
	}
	if bs.VertexAdapter != nil {
		_ = bs.VertexAdapter.Close()
	}
	if bs.Firestore != nil {
		if err := bs.Firestore.Close(); err != nil && bs.Log != nil {
			bs.Log.Error("firestore close failed", "error", err)
		}
	}
	if bs.KMS != nil {
		if err := bs.KMS.Close(); err != nil && bs.Log != nil {
			bs.Log.Error("kms close failed", "error", err)
		}
	}
}
