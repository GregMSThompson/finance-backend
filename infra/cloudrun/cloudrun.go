package cloudrun

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// SetupCloudRun enables the Cloud Run API for the current project.
func SetupCloudRun(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudRunService", &projects.ServiceArgs{
		Service: pulumi.String("run.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func createServiceAccount(ctx *pulumi.Context, prov *gcp.Provider, resourceName, accountID, displayName string) (*serviceaccount.Account, error) {
	return serviceaccount.NewAccount(ctx, resourceName, &serviceaccount.AccountArgs{
		AccountId:   pulumi.String(accountID),
		DisplayName: pulumi.String(displayName),
	},
		pulumi.Provider(prov),
	)
}

func grantFirestoreAccess(ctx *pulumi.Context, prov *gcp.Provider, sa *serviceaccount.Account, resourceName string) error {
	return grantProjectRole(ctx, prov, sa, resourceName, "roles/datastore.user")
}

func grantProjectRole(ctx *pulumi.Context, prov *gcp.Provider, sa *serviceaccount.Account, resourceName, role string) error {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	_, err := projects.NewIAMMember(ctx, resourceName, &projects.IAMMemberArgs{
		Project: pulumi.String(projectID),
		Role:    pulumi.String(role),
		Member:  serviceAccountMember(sa.Email),
	},
		pulumi.Provider(prov),
	)
	return err
}

func serviceAccountMember(email pulumi.StringOutput) pulumi.StringOutput {
	return email.ApplyT(func(value string) string {
		return fmt.Sprintf("serviceAccount:%s", value)
	}).(pulumi.StringOutput)
}
