package cloudrun

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/common"
)

func SetupCloudRun(ctx *pulumi.Context, res ...pulumi.Resource) (*serviceaccount.Account, error) {
	img, err := buildApiImage(ctx, res...)
	if err != nil {
		return nil, err
	}

	srv, err := enableCloudRun(ctx)
	if err != nil {
		return nil, err
	}

	apiSA, err := createServiceAccount(ctx)
	if err != nil {
		return nil, err
	}

	svc, err := createCloudRunService(ctx, img, apiSA, srv)
	if err != nil {
		return nil, err
	}

	err = setIAMAccessPolicy(ctx, svc)
	if err != nil {
		return nil, err
	}

	return apiSA, nil
}

func buildApiImage(ctx *pulumi.Context, res ...pulumi.Resource) (*docker.Image, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")

	hash, err := common.GenerateHash("../")
	if err != nil {
		return nil, err
	}

	return docker.NewImage(ctx, "apiImage", &docker.ImageArgs{
		Build: docker.DockerBuildArgs{
			Platform:   pulumi.String("linux/amd64"),
			Context:    pulumi.String(".."),                    // build from repo root
			Dockerfile: pulumi.String("../cmd/api/Dockerfile"), // Dockerfile path relative to repo root
		},
		ImageName: pulumi.String(fmt.Sprintf("%s-docker.pkg.dev/%s/api/finance-api:%s", region, projectID, hash)),
	},
		pulumi.DependsOn(res),
	)
}

func enableCloudRun(ctx *pulumi.Context) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudRunService", &projects.ServiceArgs{
		Service: pulumi.String("run.googleapis.com"),
	})
}

func createServiceAccount(ctx *pulumi.Context) (*serviceaccount.Account, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	apiSA, err := serviceaccount.NewAccount(ctx, "apiServiceAccount", &serviceaccount.AccountArgs{
		AccountId:   pulumi.String("api-service"),
		DisplayName: pulumi.String("API Service Account"),
	})
	if err != nil {
		return nil, err
	}

	_, err = projects.NewIAMMember(ctx, "firestoreAccess", &projects.IAMMemberArgs{
		Role: pulumi.String("roles/datastore.user"), // Firestore read/write
		Member: apiSA.Email.ApplyT(func(email string) string {
			return fmt.Sprintf("serviceAccount:%s", email)
		}).(pulumi.StringOutput),
		Project: pulumi.String(projectID),
	})
	if err != nil {
		return nil, err
	}

	return apiSA, nil
}

func createCloudRunService(ctx *pulumi.Context, img *docker.Image, apiSA *serviceaccount.Account, res ...pulumi.Resource) (*cloudrun.Service, error) {
	gcpCfg := config.New(ctx, "gcp")
	crCfg := config.New(ctx, "cloudrun")

	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")
	minScale := crCfg.Require("minScale")
	maxScale := crCfg.Require("maxScale")
	cpu := crCfg.Require("cpu")
	memory := crCfg.Require("memory")
	concurrency := crCfg.Require("concurrency")
	logLevel := crCfg.Require("logLevel")
	timeout, _ := strconv.Atoi(crCfg.Require("timeout"))

	return cloudrun.NewService(ctx, "apiService", &cloudrun.ServiceArgs{
		Location: pulumi.String(region),

		Template: &cloudrun.ServiceTemplateArgs{

			Metadata: &cloudrun.ServiceTemplateMetadataArgs{
				// ---- AUTOSCALING + INSTANCE SIZE ----
				Annotations: pulumi.StringMap{
					// Enable Identity Platform (Firebase) authentication
					"run.googleapis.com/launch-stage":      pulumi.String("BETA"),
					"run.googleapis.com/identity":          pulumi.String("true"),
					"run.googleapis.com/identity-provider": pulumi.String("firebase"),

					// Autoscaling bounds
					"autoscaling.knative.dev/minScale": pulumi.String(minScale),
					"autoscaling.knative.dev/maxScale": pulumi.String(maxScale),

					// Instance sizing
					"run.googleapis.com/cpu":    pulumi.String(cpu),
					"run.googleapis.com/memory": pulumi.String(memory),

					// Allow throttling when idle (reduces cost)
					"run.googleapis.com/cpu-throttling": pulumi.String("true"),

					// Set the number of concurrent requests per container
					"run.googleapis.com/container-concurrency": pulumi.String(concurrency),
				},
			},

			Spec: &cloudrun.ServiceTemplateSpecArgs{
				ServiceAccountName: apiSA.Email,
				TimeoutSeconds:     pulumi.Int(timeout),

				Containers: cloudrun.ServiceTemplateSpecContainerArray{
					&cloudrun.ServiceTemplateSpecContainerArgs{
						Image: img.ImageName,
						Ports: cloudrun.ServiceTemplateSpecContainerPortArray{
							&cloudrun.ServiceTemplateSpecContainerPortArgs{
								ContainerPort: pulumi.Int(8080),
							},
						},
						Envs: cloudrun.ServiceTemplateSpecContainerEnvArray{
							&cloudrun.ServiceTemplateSpecContainerEnvArgs{
								Name:  pulumi.String("PROJECTID"),
								Value: pulumi.String(projectID),
							},
							&cloudrun.ServiceTemplateSpecContainerEnvArgs{
								Name:  pulumi.String("LOGLEVEL"),
								Value: pulumi.String(logLevel),
							},
						},
					},
				},
			},
		},
	},
		pulumi.DependsOn(res),
	)
}

func setIAMAccessPolicy(ctx *pulumi.Context, svc *cloudrun.Service) error {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	_, err := cloudrun.NewIamMember(ctx, "denyUnauthenticated", &cloudrun.IamMemberArgs{
		Service:  svc.Name,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),

		// Block unauthenticated IAM invocation
		Member: pulumi.String("allUsers"),

		// Condition = false → binding ignored → public users denied
		Condition: &cloudrun.IamMemberConditionArgs{
			Title:      pulumi.String("DenyUnauthenticated"),
			Expression: pulumi.String("false"),
		},
	})
	return err
}
