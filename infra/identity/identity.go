package identity

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/identityplatform"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func SetupIdentity(ctx *pulumi.Context, prov *gcp.Provider) (*identityplatform.Config, error) {
	srv, err := enableIdentityService(ctx, prov)
	if err != nil {
		return nil, err
	}

	return enableIdentityPlatform(ctx, prov, srv)
}

func enableIdentityPlatform(ctx *pulumi.Context, prov *gcp.Provider, res ...pulumi.Resource) (*identityplatform.Config, error) {
	// Enables Identity Platform on the project (firebase)
	return identityplatform.NewConfig(ctx,
		"identityPlatformConfig",
		&identityplatform.ConfigArgs{
			SignIn: &identityplatform.ConfigSignInArgs{
				Email: &identityplatform.ConfigSignInEmailArgs{
					Enabled: pulumi.Bool(true),
				},
			},
		},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
}

func enableIdentityService(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	// Enable the identity toolkit API
	return projects.NewService(ctx, "identitytoolkitAPI", &projects.ServiceArgs{
		Service: pulumi.String("identitytoolkit.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
