package job

import (
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

// evaluatorSpec captures the small set of differences between the scheduled
// evaluator jobs (alert, goal). Everything else about their deployment — the
// container env, IAM, scheduler wiring — is identical, so it lives in the shared
// Deploy method below.
type evaluatorSpec struct {
	key         string // resource-name prefix, e.g. "alert" / "goal"
	jobName     string // Cloud Run Job name, SA account id, and cmd/ image dir
	title       string // human-readable display name, e.g. "Alert Evaluator"
	scheduleKey string // job-config key for the cron schedule
	timeZoneKey string // job-config key for the schedule time zone
}

// Evaluator deploys one scheduled evaluator Cloud Run Job and its supporting IAM
// and Cloud Scheduler trigger. The notification-delivery queue is shared across
// evaluators and passed in, not created here.
type Evaluator struct {
	provider      *gcp.Provider
	dockerManager *infraDocker.Manager
	spec          evaluatorSpec
}

// NewAlertEvaluator creates the alert evaluator deployer.
func NewAlertEvaluator(prov *gcp.Provider, dockerManager *infraDocker.Manager) *Evaluator {
	return &Evaluator{
		provider:      prov,
		dockerManager: dockerManager,
		spec: evaluatorSpec{
			key:         "alert",
			jobName:     "alert-evaluator",
			title:       "Alert Evaluator",
			scheduleKey: "alertEvaluatorSchedule",
			timeZoneKey: "alertEvaluatorTimeZone",
		},
	}
}

// NewGoalEvaluator creates the goal evaluator deployer.
func NewGoalEvaluator(prov *gcp.Provider, dockerManager *infraDocker.Manager) *Evaluator {
	return &Evaluator{
		provider:      prov,
		dockerManager: dockerManager,
		spec: evaluatorSpec{
			key:         "goal",
			jobName:     "goal-evaluator",
			title:       "Goal Evaluator",
			scheduleKey: "goalEvaluatorSchedule",
			timeZoneKey: "goalEvaluatorTimeZone",
		},
	}
}

// rn prefixes a Pulumi resource name with the evaluator key (e.g. "alert" +
// "EvaluatorJob" → "alertEvaluatorJob"), keeping resource names stable per job.
func (e *Evaluator) rn(suffix string) string {
	return e.spec.key + suffix
}

// Deploy builds and deploys the evaluator Cloud Run Job and its required IAM and
// Cloud Scheduler trigger. The shared notification-delivery queue is provided by
// the caller.
func (e *Evaluator) Deploy(ctx *pulumi.Context, queue *gcpcloudtasks.Queue, workerServiceURL, workerAudience, workerServiceName pulumi.StringOutput, res ...pulumi.Resource) error {
	img, err := e.dockerManager.BuildImage(ctx, "jobs/"+e.spec.jobName, res...)
	if err != nil {
		return err
	}

	evaluatorSA, err := iam.CreateServiceAccount(ctx, e.provider, e.rn("EvaluatorServiceAccount"), e.spec.jobName, e.spec.title+" Service Account")
	if err != nil {
		return err
	}

	schedulerSA, err := iam.CreateServiceAccount(ctx, e.provider, e.rn("EvaluatorSchedulerAccount"), e.spec.jobName+"-scheduler", e.spec.title+" Scheduler Service Account")
	if err != nil {
		return err
	}

	job, err := e.createJob(ctx, img, evaluatorSA, workerServiceURL, workerAudience, queue)
	if err != nil {
		return err
	}

	iamResources, err := e.setIAMPermissions(ctx, evaluatorSA, schedulerSA, job)
	if err != nil {
		return err
	}

	workerInvoker, err := e.grantWorkerInvoker(ctx, workerServiceName, evaluatorSA)
	if err != nil {
		return err
	}

	iamResources = append(iamResources, workerInvoker)

	return e.createScheduler(ctx, job, schedulerSA, iamResources...)
}

func (e *Evaluator) createJob(ctx *pulumi.Context, img *pulumidocker.Image, evaluatorSA *serviceaccount.Account, workerServiceURL, workerAudience pulumi.StringOutput, queue *gcpcloudtasks.Queue) (*cloudrunv2.Job, error) {
	gcpCfg := config.New(ctx, "gcp")
	jobCfg := config.New(ctx, "job")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	logLevel := jobCfg.Require("logLevel")

	return cloudrunv2.NewJob(ctx, e.rn("EvaluatorJob"), &cloudrunv2.JobArgs{
		Name:     pulumi.String(e.spec.jobName),
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
		pulumi.Provider(e.provider),
		pulumi.DependsOn([]pulumi.Resource{queue}),
	)
}

func (e *Evaluator) setIAMPermissions(ctx *pulumi.Context, evaluatorSA *serviceaccount.Account, schedulerSA *serviceaccount.Account, job *cloudrunv2.Job) ([]pulumi.Resource, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")

	firestoreAccess, err := iam.GrantFirestoreAccess(ctx, e.provider, evaluatorSA, e.rn("EvaluatorFirestoreAccess"))
	if err != nil {
		return nil, err
	}

	tasksEnqueuer, err := iam.GrantProjectRole(ctx, e.provider, evaluatorSA, e.rn("EvaluatorCloudTasksAccess"), "roles/cloudtasks.enqueuer")
	if err != nil {
		return nil, err
	}

	oidcActAs, err := serviceaccount.NewIAMMember(ctx, e.rn("EvaluatorCloudTasksOidcActAs"), &serviceaccount.IAMMemberArgs{
		ServiceAccountId: evaluatorSA.Name,
		Role:             pulumi.String("roles/iam.serviceAccountUser"),
		Member:           iam.ServiceAccountMember(evaluatorSA.Email),
	},
		pulumi.Provider(e.provider),
	)
	if err != nil {
		return nil, err
	}

	schedulerInvoker, err := cloudrunv2.NewJobIamMember(ctx, e.rn("EvaluatorSchedulerInvoker"), &cloudrunv2.JobIamMemberArgs{
		Project:  pulumi.String(projectID),
		Location: pulumi.String(region),
		Name:     job.Name,
		Role:     pulumi.String("roles/run.invoker"),
		Member:   iam.ServiceAccountMember(schedulerSA.Email),
	},
		pulumi.Provider(e.provider),
	)
	if err != nil {
		return nil, err
	}

	return []pulumi.Resource{firestoreAccess, tasksEnqueuer, oidcActAs, schedulerInvoker}, nil
}

func (e *Evaluator) grantWorkerInvoker(ctx *pulumi.Context, workerServiceName pulumi.StringOutput, evaluatorSA *serviceaccount.Account) (pulumi.Resource, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	return gcpcloudrun.NewIamMember(ctx, e.rn("EvaluatorWorkerInvoker"), &gcpcloudrun.IamMemberArgs{
		Service:  workerServiceName,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),
		Member:   iam.ServiceAccountMember(evaluatorSA.Email),
	},
		pulumi.Provider(e.provider),
	)
}

func (e *Evaluator) createScheduler(ctx *pulumi.Context, job *cloudrunv2.Job, schedulerSA *serviceaccount.Account, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	jobCfg := config.New(ctx, "job")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	schedule := jobCfg.Require(e.spec.scheduleKey)
	timeZone := jobCfg.Require(e.spec.timeZoneKey)

	jobURI := pulumi.Sprintf("https://%s-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/%s/jobs/%s:run", region, projectID, job.Name)

	_, err := cloudscheduler.NewJob(ctx, e.rn("EvaluatorScheduler"), &cloudscheduler.JobArgs{
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
		pulumi.Provider(e.provider),
		pulumi.DependsOn(res),
	)
	return err
}
