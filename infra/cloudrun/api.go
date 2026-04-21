package cloudrun

import (
	"strconv"

	infraDocker "github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/iam"
	"github.com/GregMSThompson/finance-backend/infra/secret"
	pulumidocker "github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	gcpcloudrun "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	gcpkms "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/kms"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// secretRefs contains the Secret Manager references injected into the API service.
type secretRefs struct {
	PlaidClientIDName pulumi.StringInput
	PlaidSecretName   pulumi.StringInput
}

type API struct {
	provider      *gcp.Provider
	dockerManager *infraDocker.Manager
	secretManager *secret.Manager
}

// NewAPI creates an API Cloud Run deployer bound to a provider.
func NewAPI(prov *gcp.Provider, dockerManager *infraDocker.Manager, secretManager *secret.Manager) *API {
	return &API{
		provider:      prov,
		dockerManager: dockerManager,
		secretManager: secretManager,
	}
}

// Deploy builds and deploys the API Cloud Run service and its required IAM.
func (a *API) Deploy(ctx *pulumi.Context, keyID pulumi.StringInput, res ...pulumi.Resource) (*serviceaccount.Account, error) {
	img, err := a.dockerManager.BuildImage(ctx, "api", res...)
	if err != nil {
		return nil, err
	}

	sr, err := a.createSecrets(ctx)
	if err != nil {
		return nil, err
	}

	apiSA, err := iam.CreateServiceAccount(ctx, a.provider, "apiServiceAccount", "api-service", "API Service Account")
	if err != nil {
		return nil, err
	}

	iamResources, err := a.setIAMPermissions(ctx, apiSA, keyID)
	if err != nil {
		return nil, err
	}

	svc, err := a.createService(ctx, img, apiSA, sr, keyID, iamResources...)
	if err != nil {
		return nil, err
	}

	if err := a.setIAMAccessPolicy(ctx, svc); err != nil {
		return nil, err
	}

	return apiSA, nil
}

func (a *API) createSecrets(ctx *pulumi.Context) (secretRefs, error) {
	plaidCfg := config.New(ctx, "plaid")

	plaidClientID, err := a.secretManager.CreateSecret(ctx, "plaidClientIdSecret", "plaidClientId", plaidCfg.RequireSecret("clientId"))
	if err != nil {
		return secretRefs{}, err
	}

	plaidSecret, err := a.secretManager.CreateSecret(ctx, "plaidSecretSecret", "plaidSecret", plaidCfg.RequireSecret("secret"))
	if err != nil {
		return secretRefs{}, err
	}

	return secretRefs{
		PlaidClientIDName: plaidClientID,
		PlaidSecretName:   plaidSecret,
	}, nil
}

func (a *API) createService(ctx *pulumi.Context, img *pulumidocker.Image, apiSA *serviceaccount.Account, sr secretRefs, keyID pulumi.StringInput, res ...pulumi.Resource) (*gcpcloudrun.Service, error) {
	gcpCfg := config.New(ctx, "gcp")
	crCfg := config.New(ctx, "cloudrun")
	plaidCfg := config.New(ctx, "plaid")
	vertexCfg := config.New(ctx, "vertex")
	appCfg := config.New(ctx, "app")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	minScale := crCfg.Require("minScale")
	maxScale := crCfg.Require("maxScale")
	cpu := crCfg.Require("cpu")
	memory := crCfg.Require("memory")
	concurrency := crCfg.Require("concurrency")
	logLevel := crCfg.Require("logLevel")
	timeout, _ := strconv.Atoi(crCfg.Require("timeout"))
	plaidEnv := plaidCfg.Require("environment")
	vertexModel := vertexCfg.Require("model")
	aiTTL := appCfg.Require("aiTtl")

	envs := gcpcloudrun.ServiceTemplateSpecContainerEnvArray{
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("PROJECTID"),
			Value: pulumi.String(projectID),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("REGION"),
			Value: pulumi.String(region),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("LOGLEVEL"),
			Value: pulumi.String(logLevel),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("KMSKEYNAME"),
			Value: keyID,
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("PLAIDENVIRONMENT"),
			Value: pulumi.String(plaidEnv),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("VERTEXMODEL"),
			Value: pulumi.String(vertexModel),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("AITTL"),
			Value: pulumi.String(aiTTL),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name: pulumi.String("PLAIDCLIENTID"),
			ValueFrom: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromArgs{
				SecretKeyRef: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromSecretKeyRefArgs{
					Name: sr.PlaidClientIDName,
					Key:  pulumi.String("latest"),
				},
			},
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name: pulumi.String("PLAIDSECRET"),
			ValueFrom: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromArgs{
				SecretKeyRef: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromSecretKeyRefArgs{
					Name: sr.PlaidSecretName,
					Key:  pulumi.String("latest"),
				},
			},
		},
	}

	return gcpcloudrun.NewService(ctx, "apiService", &gcpcloudrun.ServiceArgs{
		Location: pulumi.String(region),
		Template: &gcpcloudrun.ServiceTemplateArgs{
			Metadata: &gcpcloudrun.ServiceTemplateMetadataArgs{
				Annotations: pulumi.StringMap{
					"autoscaling.knative.dev/minScale":         pulumi.String(minScale),
					"autoscaling.knative.dev/maxScale":         pulumi.String(maxScale),
					"run.googleapis.com/cpu":                   pulumi.String(cpu),
					"run.googleapis.com/memory":                pulumi.String(memory),
					"run.googleapis.com/cpu-throttling":        pulumi.String("true"),
					"run.googleapis.com/container-concurrency": pulumi.String(concurrency),
				},
			},
			Spec: &gcpcloudrun.ServiceTemplateSpecArgs{
				ServiceAccountName: apiSA.Email,
				TimeoutSeconds:     pulumi.Int(timeout),
				Containers: gcpcloudrun.ServiceTemplateSpecContainerArray{
					&gcpcloudrun.ServiceTemplateSpecContainerArgs{
						Image: img.ImageName,
						Ports: gcpcloudrun.ServiceTemplateSpecContainerPortArray{
							&gcpcloudrun.ServiceTemplateSpecContainerPortArgs{
								ContainerPort: pulumi.Int(8080),
							},
						},
						Envs: envs,
					},
				},
			},
		},
	},
		pulumi.Provider(a.provider),
		pulumi.DependsOn(res),
	)
}

func (a *API) setIAMAccessPolicy(ctx *pulumi.Context, svc *gcpcloudrun.Service) error {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	_, err := gcpcloudrun.NewIamMember(ctx, "denyUnauthenticated", &gcpcloudrun.IamMemberArgs{
		Service:  svc.Name,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),
		Member:   pulumi.String("allUsers"),
	},
		pulumi.Provider(a.provider),
	)
	return err
}

func (a *API) setIAMPermissions(ctx *pulumi.Context, apiSA *serviceaccount.Account, keyID pulumi.StringInput) ([]pulumi.Resource, error) {
	firestoreAccess, err := iam.GrantFirestoreAccess(ctx, a.provider, apiSA, "apiFirestoreAccess")
	if err != nil {
		return nil, err
	}

	secretAccess, err := iam.GrantProjectRole(ctx, a.provider, apiSA, "apiSecretManagerAccess", "roles/secretmanager.secretAccessor")
	if err != nil {
		return nil, err
	}

	kmsAccess, err := gcpkms.NewCryptoKeyIAMMember(ctx, "apiKMSKeyAccess", &gcpkms.CryptoKeyIAMMemberArgs{
		CryptoKeyId: keyID,
		Role:        pulumi.String("roles/cloudkms.cryptoKeyEncrypterDecrypter"),
		Member:      iam.ServiceAccountMember(apiSA.Email),
	},
		pulumi.Provider(a.provider),
	)
	if err != nil {
		return nil, err
	}

	vertexAccess, err := iam.GrantProjectRole(ctx, a.provider, apiSA, "apiVertexAccess", "roles/aiplatform.user")
	if err != nil {
		return nil, err
	}

	return []pulumi.Resource{
		firestoreAccess,
		secretAccess,
		kmsAccess,
		vertexAccess,
	}, nil
}
