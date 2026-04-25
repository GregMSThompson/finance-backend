package cloudrun

import (
	"fmt"
	"strconv"

	infraDocker "github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/iam"
	"github.com/GregMSThompson/finance-backend/infra/secret"
	pulumidocker "github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	gcpcloudrun "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Worker deploys the Cloud Run worker service and its required IAM.
type Worker struct {
	provider      *gcp.Provider
	dockerManager *infraDocker.Manager
	secretManager *secret.Manager
}

// WorkerDeployment contains the deployed worker service resources needed by other stacks.
type WorkerDeployment struct {
	Service        *gcpcloudrun.Service
	ServiceAccount *serviceaccount.Account
	Audience       string
}

// NewWorker creates a worker Cloud Run deployer bound to a provider.
func NewWorker(prov *gcp.Provider, dockerManager *infraDocker.Manager, secretManager *secret.Manager) *Worker {
	return &Worker{
		provider:      prov,
		dockerManager: dockerManager,
		secretManager: secretManager,
	}
}

// Deploy builds and deploys the worker Cloud Run service and its required IAM.
func (w *Worker) Deploy(ctx *pulumi.Context, res ...pulumi.Resource) (*WorkerDeployment, error) {
	img, err := w.dockerManager.BuildImage(ctx, "worker", res...)
	if err != nil {
		return nil, err
	}

	workerSA, err := iam.CreateServiceAccount(ctx, w.provider, "workerServiceAccount", "worker-service", "Worker Service Account")
	if err != nil {
		return nil, err
	}

	iamResources, err := w.setIAMPermissions(ctx, workerSA)
	if err != nil {
		return nil, err
	}

	svc, err := w.createService(ctx, img, workerSA, iamResources...)
	if err != nil {
		return nil, err
	}

	if err := w.setIAMAccessPolicy(ctx, svc); err != nil {
		return nil, err
	}

	appCfg := config.New(ctx, "app")
	workerAudience := appCfg.Require("workerAudience")

	return &WorkerDeployment{
		Service:        svc,
		ServiceAccount: workerSA,
		Audience:       workerAudience,
	}, exportServiceURL(ctx, "workerServiceURL", svc)
}

func (w *Worker) createService(ctx *pulumi.Context, img *pulumidocker.Image, workerSA *serviceaccount.Account, res ...pulumi.Resource) (*gcpcloudrun.Service, error) {
	gcpCfg := config.New(ctx, "gcp")
	crCfg := config.New(ctx, "cloudrun")
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
	appEnv := ctx.Stack()
	workerAudience := appCfg.Require("workerAudience")
	workerTestAPIKeyEnabled := appCfg.GetBool("workerTestApiKeyEnabled")

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
			Name:  pulumi.String("APPENV"),
			Value: pulumi.String(appEnv),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("WORKERAUDIENCE"),
			Value: pulumi.String(workerAudience),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("WORKERTESTAPIKEYENABLED"),
			Value: pulumi.String(strconv.FormatBool(workerTestAPIKeyEnabled)),
		},
	}

	if workerTestAPIKeyEnabled {
		workerTestAPIKey, err := w.secretManager.CreateSecret(
			ctx,
			"workerTestApiKeySecret",
			"workerTestApiKey",
			appCfg.RequireSecret("workerTestApiKey"),
		)
		if err != nil {
			return nil, err
		}

		envs = append(envs, &gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name: pulumi.String("WORKERTESTAPIKEY"),
			ValueFrom: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromArgs{
				SecretKeyRef: &gcpcloudrun.ServiceTemplateSpecContainerEnvValueFromSecretKeyRefArgs{
					Name: workerTestAPIKey,
					Key:  pulumi.String("latest"),
				},
			},
		})
	}

	return gcpcloudrun.NewService(ctx, "workerService", &gcpcloudrun.ServiceArgs{
		Location: pulumi.String(region),
		Metadata: &gcpcloudrun.ServiceMetadataArgs{
			Annotations: pulumi.StringMap{
				"run.googleapis.com/custom-audiences": pulumi.String(fmt.Sprintf("[\"%s\"]", workerAudience)),
			},
		},
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
				ServiceAccountName: workerSA.Email,
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
		pulumi.Provider(w.provider),
		pulumi.DependsOn(res),
	)
}

func (w *Worker) setIAMPermissions(ctx *pulumi.Context, workerSA *serviceaccount.Account) ([]pulumi.Resource, error) {
	firestoreAccess, err := iam.GrantFirestoreAccess(ctx, w.provider, workerSA, "workerFirestoreAccess")
	if err != nil {
		return nil, err
	}

	secretAccess, err := iam.GrantProjectRole(ctx, w.provider, workerSA, "workerSecretManagerAccess", "roles/secretmanager.secretAccessor")
	if err != nil {
		return nil, err
	}

	firebaseMessagingAccess, err := iam.GrantProjectRole(ctx, w.provider, workerSA, "workerFirebaseMessagingAccess", "roles/firebasecloudmessaging.admin")
	if err != nil {
		return nil, err
	}

	return []pulumi.Resource{
		firestoreAccess,
		secretAccess,
		firebaseMessagingAccess,
	}, nil
}

func (w *Worker) setIAMAccessPolicy(ctx *pulumi.Context, svc *gcpcloudrun.Service) error {
	gcpCfg := config.New(ctx, "gcp")
	appCfg := config.New(ctx, "app")

	if !appCfg.GetBool("workerAllowPublicInvoker") {
		return nil
	}

	region := gcpCfg.Require("region")

	_, err := gcpcloudrun.NewIamMember(ctx, "workerAllowUnauthenticated", &gcpcloudrun.IamMemberArgs{
		Service:  svc.Name,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),
		Member:   pulumi.String("allUsers"),
	},
		pulumi.Provider(w.provider),
	)
	return err
}
