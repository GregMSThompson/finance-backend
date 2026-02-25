package vertex

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func SetupVertex(ctx *pulumi.Context, prov *gcp.Provider) error {
	_, err := enableVertex(ctx, prov)
	return err
}

func enableVertex(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "vertex", &projects.ServiceArgs{
		Service: pulumi.String("aiplatform.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
