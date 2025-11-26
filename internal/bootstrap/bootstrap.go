package bootstrap

import (
	"context"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"

	"github.com/GregMSThompson/finance-backend/internal/config"
)

type Bootstrap struct {
	Firestore *firestore.Client
	Firebase  *auth.Client
}

func Run(cfg *config.Config) (*Bootstrap, error) {
	startupCtx := context.Background()

	fs, err := InitFirestore(startupCtx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}
	fb, err := InitFirebase(startupCtx)
	if err != nil {
		return nil, err
	}

	return &Bootstrap{
		Firestore: fs,
		Firebase:  fb,
	}, nil
}
