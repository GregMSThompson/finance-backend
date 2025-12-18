package docker

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateCloudrunRepo(ctx *pulumi.Context) (*artifactregistry.Repository, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	return artifactregistry.NewRepository(ctx, "apiRepository", &artifactregistry.RepositoryArgs{
		Format:       pulumi.String("DOCKER"),
		RepositoryId: pulumi.String("api"),
		Location:     pulumi.String(region),
		Description:  pulumi.String("Docker repository for API images"),
	})
}
