package bootstrap

import (
	"context"
	"log/slog"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type Bootstrap struct {
	Log       *slog.Logger
	Firestore *firestore.Client
	Firebase  *auth.Client
}

func Run(cfg *config.Config) (*Bootstrap, error) {
	var err error
	applicationCtx := context.Background()
	bs := new(Bootstrap)

	bs.Log = logger.New(cfg.LogLevel, logger.NewCloudRunHandler)
	bs.Firestore, err = InitFirestore(applicationCtx, cfg.ProjectID)
	if err != nil {
		return bs, err
	}
	bs.Firebase, err = InitFirebase(applicationCtx)
	if err != nil {
		return bs, err
	}

	return bs, nil
}
