package firestore

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/firestore"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func SetupFirestore(ctx *pulumi.Context) error {
	if err := enableFireStore(ctx); err != nil {
		return err
	}

	if err := createDatabase(ctx); err != nil {
		return err
	}

	return nil
}

func enableFireStore(ctx *pulumi.Context) error {
	_, err := projects.NewService(ctx, "firestore", &projects.ServiceArgs{
		Service: pulumi.String("firestore.googleapis.com"),
	})
	return err
}

func createDatabase(ctx *pulumi.Context) error {
	cfg := config.New(ctx, "")
	projectID := cfg.Require("gcp:project")
	region := cfg.Require(("gcp:region"))

	_, err := firestore.NewDatabase(ctx, "firestoreDatabase", &firestore.DatabaseArgs{
		Project:    pulumi.String(projectID),
		LocationId: pulumi.String(region),
		Type:       pulumi.String("FIRESTORE_NATIVE"),
	})
	return err
}
