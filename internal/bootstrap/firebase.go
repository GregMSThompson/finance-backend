package bootstrap

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

func InitFirebase(ctx context.Context, projectID string) (*auth.Client, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}
	return app.Auth(ctx)
}
