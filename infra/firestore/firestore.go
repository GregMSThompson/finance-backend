package firestore

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/firestore"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func SetupFirestore(ctx *pulumi.Context) error {
	svc, err := enableFireStore(ctx)
	if err != nil {
		return err
	}

	if err := createDatabase(ctx, svc); err != nil {
		return err
	}

	return nil
}

func enableFireStore(ctx *pulumi.Context) (*projects.Service, error) {
	return projects.NewService(ctx, "firestore", &projects.ServiceArgs{
		Service: pulumi.String("firestore.googleapis.com"),
	})
}

func createDatabase(ctx *pulumi.Context, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require(("region"))

	_, err := firestore.NewDatabase(ctx, "firestoreDatabase", &firestore.DatabaseArgs{
		Project:    pulumi.String(projectID),
		LocationId: pulumi.String(region),
		Type:       pulumi.String("FIRESTORE_NATIVE"),
	},
		pulumi.DependsOn(res),
	)
	return err
}
