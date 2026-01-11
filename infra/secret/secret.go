package secret

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/secretmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	provider *gcp.Provider
	service  *projects.Service
)

func AddSecret(ctx *pulumi.Context,
	resourceName,
	secretID string,
	value pulumi.StringInput) (pulumi.StringOutput, error) {
	emptyString := pulumi.String("").ToStringOutput()
	s, err := secretmanager.NewSecret(ctx, resourceName, &secretmanager.SecretArgs{
		SecretId: pulumi.String(secretID),
		Replication: &secretmanager.SecretReplicationArgs{
			Auto: &secretmanager.SecretReplicationAutoArgs{},
		},
	},
		pulumi.Provider(provider),
		pulumi.DependsOn([]pulumi.Resource{service}),
	)
	if err != nil {
		return emptyString, err
	}

	_, err = secretmanager.NewSecretVersion(ctx, resourceName+"Version", &secretmanager.SecretVersionArgs{
		Secret:     s.ID(),
		SecretData: value,
	},
		pulumi.Provider(provider),
	)
	if err != nil {
		return emptyString, err
	}

	return s.SecretId, nil
}

func SetupSecretManager(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	var err error
	service, err = enableSecretsManager(ctx, prov)
	if err != nil {
		return nil, err
	}

	provider = prov
	return service, nil
}

func enableSecretsManager(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "secretManagerService", &projects.ServiceArgs{
		Service: pulumi.String("secretmanager.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}
