package cloudtasks

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SetupCloudTasks enables the Cloud Tasks API for the current project.
func SetupCloudTasks(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudTasksService", &projects.ServiceArgs{
		Service: pulumi.String("cloudtasks.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
