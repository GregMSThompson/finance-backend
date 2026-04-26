package cloudscheduler

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SetupCloudScheduler enables the Cloud Scheduler API for the current project.
func SetupCloudScheduler(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudSchedulerService", &projects.ServiceArgs{
		Service: pulumi.String("cloudscheduler.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
