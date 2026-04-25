package job

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudtasks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func createCloudTasksQueue(ctx *pulumi.Context, prov *gcp.Provider, resourceName, queueName string, res ...pulumi.Resource) (*cloudtasks.Queue, error) {
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