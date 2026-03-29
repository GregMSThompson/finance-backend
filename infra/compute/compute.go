package compute

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SetupCompute enables the Compute Engine API for the current project.
func SetupCompute(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "computeService", &projects.ServiceArgs{
		Service: pulumi.String("compute.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
