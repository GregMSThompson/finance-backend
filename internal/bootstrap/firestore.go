package bootstrap

import (
	"context"

	"cloud.google.com/go/firestore"
)

func InitFirestore(ctx context.Context, projectID string) (*firestore.Client, error) {
	return firestore.NewClient(ctx, projectID)
}
