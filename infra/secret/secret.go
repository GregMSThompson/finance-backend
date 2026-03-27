package secret

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/secretmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Manager struct {
	provider *gcp.Provider
}

func New(prov *gcp.Provider) *Manager {
	return &Manager{provider: prov}
}

func SetupSecretManager(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "secretManagerService", &projects.ServiceArgs{
		Service: pulumi.String("secretmanager.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func (m *Manager) CreateSecret(ctx *pulumi.Context,
	resourceName,
	secretID string,
	value pulumi.StringInput,
	res ...pulumi.Resource) (pulumi.StringOutput, error) {
	emptyString := pulumi.String("").ToStringOutput()
	s, err := secretmanager.NewSecret(ctx, resourceName, &secretmanager.SecretArgs{
		SecretId: pulumi.String(secretID),
		Replication: &secretmanager.SecretReplicationArgs{
			Auto: &secretmanager.SecretReplicationAutoArgs{},
		},
	},
		pulumi.Provider(m.provider),
		pulumi.DependsOn(res),
	)
	if err != nil {
		return emptyString, err
	}

	_, err = secretmanager.NewSecretVersion(ctx, resourceName+"Version", &secretmanager.SecretVersionArgs{
		Secret:     s.ID(),
		SecretData: value,
	},
		pulumi.Provider(m.provider),
	)
	if err != nil {
		return emptyString, err
	}

	return s.SecretId, nil
}
