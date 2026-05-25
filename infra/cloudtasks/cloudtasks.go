package cloudtasks

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudtasks"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	// JobsQueueName is the Cloud Tasks queue used by the API to dispatch worker jobs.
	JobsQueueName = "jobs"
	// AlertDeliveryQueueName is the Cloud Tasks queue used by the alert evaluator to dispatch deliveries.
	AlertDeliveryQueueName = "alert-delivery"
)

// SetupCloudTasks enables the Cloud Tasks API for the current project.
func SetupCloudTasks(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudTasksService", &projects.ServiceArgs{
		Service: pulumi.String("cloudtasks.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

// CreateQueue provisions a Cloud Tasks queue in the configured GCP region.
func CreateQueue(ctx *pulumi.Context, prov *gcp.Provider, resourceName, queueName string, res ...pulumi.Resource) (*cloudtasks.Queue, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	return cloudtasks.NewQueue(ctx, resourceName, &cloudtasks.QueueArgs{
		Name:     pulumi.String(queueName),
		Location: pulumi.String(region),
	},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
}
