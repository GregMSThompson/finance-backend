package job

import (
	infracloudtasks "github.com/GregMSThompson/finance-backend/infra/cloudtasks"
	infraDocker "github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/iam"
	pulumidocker "github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	gcpcloudrun "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrunv2"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudscheduler"
	gcpcloudtasks "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudtasks"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// AlertEvaluator deploys the alert evaluator Cloud Run Job and its required infrastructure.
type AlertEvaluator struct {
	provider      *gcp.Provider
	dockerManager *infraDocker.Manager
}

// NewAlertEvaluator creates an alert evaluator deployer bound to a provider.
func NewAlertEvaluator(prov *gcp.Provider, dockerManager *infraDocker.Manager) *AlertEvaluator {
	return &AlertEvaluator{
		provider:      prov,
		dockerManager: dockerManager,
	}
}

// Deploy builds and deploys the alert evaluator Cloud Run Job and its required IAM,
// Cloud Tasks queue, and Cloud Scheduler trigger.
func (a *AlertEvaluator) Deploy(ctx *pulumi.Context, workerServiceURL, workerAudience, workerServiceName pulumi.StringOutput, res ...pulumi.Resource) error {
	img, err := a.dockerManager.BuildImage(ctx, "jobs/alert-evaluator", res...)
	if err != nil {
		return err
	}

	evaluatorSA, err := iam.CreateServiceAccount(ctx, a.provider, "alertEvaluatorServiceAccount", "alert-evaluator", "Alert Evaluator Service Account")
	if err != nil {
		return err
	}

	queue, err := infracloudtasks.CreateQueue(ctx, a.provider, "alertDeliveryQueue", infracloudtasks.AlertDeliveryQueueName)
	if err != nil {
		return err
	}

	schedulerSA, err := iam.CreateServiceAccount(ctx, a.provider, "alertEvaluatorSchedulerAccount", "alert-evaluator-scheduler", "Alert Evaluator Scheduler Service Account")
	if err != nil {
		return err
	}

	job, err := a.createJob(ctx, img, evaluatorSA, workerServiceURL, workerAudience, queue)
	if err != nil {
		return err
	}

	iamResources, err := a.setIAMPermissions(ctx, evaluatorSA, schedulerSA, job)
	if err != nil {
		return err
	}

	workerInvoker, err := a.grantWorkerInvoker(ctx, workerServiceName, evaluatorSA)
	if err != nil {
		return err
	}

	iamResources = append(iamResources, workerInvoker)

	if err := a.createScheduler(ctx, job, schedulerSA, iamResources...); err != nil {
		return err
	}

	ctx.Export("alertDeliveryQueueName", queue.Name)

	return nil
}

func (a *AlertEvaluator) createJob(ctx *pulumi.Context, img *pulumidocker.Image, evaluatorSA *serviceaccount.Account, workerServiceURL, workerAudience pulumi.StringOutput, queue *gcpcloudtasks.Queue) (*cloudrunv2.Job, error) {
	gcpCfg := config.New(ctx, "gcp")
	jobCfg := config.New(ctx, "job")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	logLevel := jobCfg.Require("logLevel")

	return cloudrunv2.NewJob(ctx, "alertEvaluatorJob", &cloudrunv2.JobArgs{
		Name:     pulumi.String("alert-evaluator"),
		Location: pulumi.String(region),
		Template: &cloudrunv2.JobTemplateArgs{
			Template: &cloudrunv2.JobTemplateTemplateArgs{
				ServiceAccount: evaluatorSA.Email,
				Containers: cloudrunv2.JobTemplateTemplateContainerArray{
					&cloudrunv2.JobTemplateTemplateContainerArgs{
						Image: img.ImageName,
						Envs: cloudrunv2.JobTemplateTemplateContainerEnvArray{
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("PROJECTID"),
								Value: pulumi.String(projectID),
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("REGION"),
								Value: pulumi.String(region),
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("LOGLEVEL"),
								Value: pulumi.String(logLevel),
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("CLOUDTASKSQUEUE"),
								Value: queue.Name,
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("WORKERURL"),
								Value: workerServiceURL,
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("WORKERAUDIENCE"),
								Value: workerAudience,
							},
							&cloudrunv2.JobTemplateTemplateContainerEnvArgs{
								Name:  pulumi.String("WORKERSERVICEACCT"),
								Value: evaluatorSA.Email,
							},
						},
					},
				},
			},
		},
	},
		pulumi.Provider(a.provider),
		pulumi.DependsOn([]pulumi.Resource{queue}),
	)
}

func (a *AlertEvaluator) setIAMPermissions(ctx *pulumi.Context, evaluatorSA *serviceaccount.Account, schedulerSA *serviceaccount.Account, job *cloudrunv2.Job) ([]pulumi.Resource, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")

	firestoreAccess, err := iam.GrantFirestoreAccess(ctx, a.provider, evaluatorSA, "alertEvaluatorFirestoreAccess")
	if err != nil {
		return nil, err
	}

	tasksEnqueuer, err := iam.GrantProjectRole(ctx, a.provider, evaluatorSA, "alertEvaluatorCloudTasksAccess", "roles/cloudtasks.enqueuer")
	if err != nil {
		return nil, err
	}

	oidcActAs, err := serviceaccount.NewIAMMember(ctx, "alertEvaluatorCloudTasksOidcActAs", &serviceaccount.IAMMemberArgs{
		ServiceAccountId: evaluatorSA.Name,
		Role:             pulumi.String("roles/iam.serviceAccountUser"),
		Member:           iam.ServiceAccountMember(evaluatorSA.Email),
	},
		pulumi.Provider(a.provider),
	)
	if err != nil {
		return nil, err
	}

	schedulerInvoker, err := cloudrunv2.NewJobIamMember(ctx, "alertEvaluatorSchedulerInvoker", &cloudrunv2.JobIamMemberArgs{
		Project:  pulumi.String(projectID),
		Location: pulumi.String(region),
		Name:     job.Name,
		Role:     pulumi.String("roles/run.invoker"),
		Member:   iam.ServiceAccountMember(schedulerSA.Email),
	},
		pulumi.Provider(a.provider),
	)
	if err != nil {
		return nil, err
	}

	return []pulumi.Resource{firestoreAccess, tasksEnqueuer, oidcActAs, schedulerInvoker}, nil
}

func (a *AlertEvaluator) grantWorkerInvoker(ctx *pulumi.Context, workerServiceName pulumi.StringOutput, evaluatorSA *serviceaccount.Account) (pulumi.Resource, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	return gcpcloudrun.NewIamMember(ctx, "alertEvaluatorWorkerInvoker", &gcpcloudrun.IamMemberArgs{
		Service:  workerServiceName,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),
		Member:   iam.ServiceAccountMember(evaluatorSA.Email),
	},
		pulumi.Provider(a.provider),
	)
}

func (a *AlertEvaluator) createScheduler(ctx *pulumi.Context, job *cloudrunv2.Job, schedulerSA *serviceaccount.Account, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	jobCfg := config.New(ctx, "job")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	schedule := jobCfg.Require("alertEvaluatorSchedule")
	timeZone := jobCfg.Require("alertEvaluatorTimeZone")

	jobURI := pulumi.Sprintf("https://%s-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/%s/jobs/%s:run", region, projectID, job.Name)

	_, err := cloudscheduler.NewJob(ctx, "alertEvaluatorScheduler", &cloudscheduler.JobArgs{
		Region:   pulumi.String(region),
		Schedule: pulumi.String(schedule),
		TimeZone: pulumi.String(timeZone),
		HttpTarget: &cloudscheduler.JobHttpTargetArgs{
			Uri:        jobURI,
			HttpMethod: pulumi.String("POST"),
			OauthToken: &cloudscheduler.JobHttpTargetOauthTokenArgs{
				ServiceAccountEmail: schedulerSA.Email,
			},
		},
	},
		pulumi.Provider(a.provider),
		pulumi.DependsOn(res),
	)
	return err
}
