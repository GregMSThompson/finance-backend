package identity

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/identityplatform"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func SetupIdentity(ctx *pulumi.Context) (*identityplatform.Config, error) {
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
	)
}
