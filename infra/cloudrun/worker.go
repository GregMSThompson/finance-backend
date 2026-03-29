package cloudrun

import (
	"strconv"

	infraDocker "github.com/GregMSThompson/finance-backend/infra/docker"
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
}

// NewWorker creates a worker Cloud Run deployer bound to a provider.
func NewWorker(prov *gcp.Provider, dockerManager *infraDocker.Manager) *Worker {
	return &Worker{
		provider:      prov,
		dockerManager: dockerManager,
	}
}

// Deploy builds and deploys the worker Cloud Run service and its required IAM.
func (w *Worker) Deploy(ctx *pulumi.Context, res ...pulumi.Resource) (*serviceaccount.Account, error) {
	img, err := w.dockerManager.BuildImage(ctx, "worker", res...)
	if err != nil {
		return nil, err
	}

	workerSA, err := createServiceAccount(ctx, w.provider, "workerServiceAccount", "worker-service", "Worker Service Account")
	if err != nil {
		return nil, err
	}

	svc, err := w.createService(ctx, img, workerSA)
	if err != nil {
		return nil, err
	}

	if err := w.setIAMPermissions(ctx, workerSA); err != nil {
		return nil, err
	}

	return workerSA, exportServiceURL(ctx, "workerServiceURL", svc)
}

func (w *Worker) createService(ctx *pulumi.Context, img *pulumidocker.Image, workerSA *serviceaccount.Account) (*gcpcloudrun.Service, error) {
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
	workerAudience := appCfg.Require("workerAudience")
	workerTestAPIKey := appCfg.GetSecret("workerTestApiKey")

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
			Name:  pulumi.String("WORKERAUDIENCE"),
			Value: pulumi.String(workerAudience),
		},
		&gcpcloudrun.ServiceTemplateSpecContainerEnvArgs{
			Name:  pulumi.String("WORKERTESTAPIKEY"),
			Value: workerTestAPIKey,
		},
	}

	return gcpcloudrun.NewService(ctx, "workerService", &gcpcloudrun.ServiceArgs{
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
	)
}

func (w *Worker) setIAMPermissions(ctx *pulumi.Context, workerSA *serviceaccount.Account) error {
	if err := grantFirestoreAccess(ctx, w.provider, workerSA, "workerFirestoreAccess"); err != nil {
		return err
	}

	return grantProjectRole(ctx, w.provider, workerSA, "workerFirebaseMessagingAccess", "roles/firebasecloudmessaging.admin")
}
