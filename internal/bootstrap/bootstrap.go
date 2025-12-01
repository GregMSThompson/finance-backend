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
	applicationCtx := context.Background()

	log := logger.New(cfg.LogLevel, logger.NewCloudRunHandler)
	fs, err := InitFirestore(applicationCtx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}
	fb, err := InitFirebase(applicationCtx)
	if err != nil {
		return nil, err
	}

	return &Bootstrap{
		Log:       log,
		Firestore: fs,
		Firebase:  fb,
	}, nil
}
