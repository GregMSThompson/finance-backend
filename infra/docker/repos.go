package docker

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateCloudrunRepo(ctx *pulumi.Context) (*artifactregistry.Repository, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	srv, err := enableArtifactRegistry(ctx)
	if err != nil {
		return nil, err
	}

	return artifactregistry.NewRepository(ctx, "apiRepository", &artifactregistry.RepositoryArgs{
		Format:       pulumi.String("DOCKER"),
		RepositoryId: pulumi.String("api"),
		Location:     pulumi.String(region),
		Description:  pulumi.String("Docker repository for API images"),
	},
		pulumi.DependsOn([]pulumi.Resource{srv}),
	)
}

func enableArtifactRegistry(ctx *pulumi.Context) (*projects.Service, error) {
	return projects.NewService(ctx, "artifactRegistry", &projects.ServiceArgs{
		Service: pulumi.String("artifactregistry.googleapis.com"),
	})
}
