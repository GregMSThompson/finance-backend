package cloudrun

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	gcpcloudrun "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SetupCloudRun enables the Cloud Run API for the current project.
func SetupCloudRun(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudRunService", &projects.ServiceArgs{
		Service: pulumi.String("run.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func exportServiceURL(ctx *pulumi.Context, name string, svc *gcpcloudrun.Service) error {
	ctx.Export(name, svc.Statuses.Index(pulumi.Int(0)).Url())
	return nil
}
