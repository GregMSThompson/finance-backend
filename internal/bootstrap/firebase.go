package bootstrap

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
)

type firebaseClients struct {
	Auth      *auth.Client
	Messaging *messaging.Client
}

// initFirebase initialises the Firebase app and returns both the auth and messaging clients.
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
