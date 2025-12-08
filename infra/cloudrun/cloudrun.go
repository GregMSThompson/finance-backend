package cloudrun

import (
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func SetupCloudRun(ctx *pulumi.Context) error {
	img, err := buildApiImage(ctx)
	if err != nil {
		return err
	}
}

func buildApiImage(ctx *pulumi.Context) (*docker.Image, error) {
	return docker.NewImage(ctx, "apiImage", &docker.ImageArgs{
		Build: docker.DockerBuildArgs{
			Platform:   pulumi.String("linux/amd64"),
			Context:    pulumi.String("./"),
			Dockerfile: pulumi.String("./cmd/api/Dockerfile"),
		},
		ImageName: pulumi.String("us-central1-docker.pkg.dev/finance-backend/api:latest"),
	})
}
