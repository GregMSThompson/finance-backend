package provider

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func SetupDefaultProvider(ctx *pulumi.Context) (*gcp.Provider, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")

	return gcp.NewProvider(ctx, "gcpProvider", &gcp.ProviderArgs{
		Project:             pulumi.String(projectID),
		Region:              pulumi.String(region),
		UserProjectOverride: pulumi.Bool(true),
	})
}
