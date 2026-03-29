package fcm

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SetupFCM enables the Firebase Cloud Messaging API for the current project.
func SetupFCM(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "firebaseCloudMessagingService", &projects.ServiceArgs{
		Service: pulumi.String("fcm.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
