package firestore

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/firestore"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func SetupFirestore(ctx *pulumi.Context, prov *gcp.Provider) error {
	svc, err := enableFireStore(ctx, prov)
	if err != nil {
		return err
	}

	if err := createDatabase(ctx, prov, svc); err != nil {
		return err
	}

	return nil
}

func enableFireStore(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "firestore", &projects.ServiceArgs{
		Service: pulumi.String("firestore.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func createDatabase(ctx *pulumi.Context, prov *gcp.Provider, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require(("region"))

	_, err := firestore.NewDatabase(ctx, "firestoreDatabase", &firestore.DatabaseArgs{
		Project:    pulumi.String(projectID),
		LocationId: pulumi.String(region),
		Type:       pulumi.String("FIRESTORE_NATIVE"),
	},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
	return err
}
