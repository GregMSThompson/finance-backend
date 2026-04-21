package iam

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// CreateServiceAccount creates a GCP service account.
func CreateServiceAccount(ctx *pulumi.Context, prov *gcp.Provider, resourceName, accountID, displayName string) (*serviceaccount.Account, error) {
	return serviceaccount.NewAccount(ctx, resourceName, &serviceaccount.AccountArgs{
		AccountId:   pulumi.String(accountID),
		DisplayName: pulumi.String(displayName),
	},
		pulumi.Provider(prov),
	)
}

// GrantFirestoreAccess grants the service account the Firestore user role.
func GrantFirestoreAccess(ctx *pulumi.Context, prov *gcp.Provider, sa *serviceaccount.Account, resourceName string) (pulumi.Resource, error) {
	return GrantProjectRole(ctx, prov, sa, resourceName, "roles/datastore.user")
}

// GrantProjectRole grants the service account a project-level IAM role.
func GrantProjectRole(ctx *pulumi.Context, prov *gcp.Provider, sa *serviceaccount.Account, resourceName, role string) (pulumi.Resource, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	return projects.NewIAMMember(ctx, resourceName, &projects.IAMMemberArgs{
		Project: pulumi.String(projectID),
		Role:    pulumi.String(role),
		Member:  ServiceAccountMember(sa.Email),
	},
		pulumi.Provider(prov),
	)
}

// ServiceAccountMember formats a service account email as an IAM member string.
func ServiceAccountMember(email pulumi.StringOutput) pulumi.StringOutput {
	return email.ApplyT(func(value string) string {
		return fmt.Sprintf("serviceAccount:%s", value)
	}).(pulumi.StringOutput)
}
