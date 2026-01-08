package cloudrun

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/common"
	"github.com/GregMSThompson/finance-backend/infra/secret"
)

type secretRefs struct {
	plaidClientIDName pulumi.StringOutput
	plaidSecretName   pulumi.StringOutput
}

func SetupCloudRun(ctx *pulumi.Context, prov *gcp.Provider, res ...pulumi.Resource) (*serviceaccount.Account, error) {
	img, err := buildApiImage(ctx, res...)
	if err != nil {
		return nil, err
	}

	sr, err := createSecrets(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := enableCloudRun(ctx, prov)
	if err != nil {
		return nil, err
	}

	apiSA, err := createServiceAccount(ctx, prov)
	if err != nil {
		return nil, err
	}

	svc, err := createCloudRunService(ctx, img, apiSA, sr, prov, srv)
	if err != nil {
		return nil, err
	}

	err = setIAMAccessPolicy(ctx, svc, prov)
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

func enableCloudRun(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "cloudRunService", &projects.ServiceArgs{
		Service: pulumi.String("run.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func createServiceAccount(ctx *pulumi.Context, prov *gcp.Provider) (*serviceaccount.Account, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	apiSA, err := serviceaccount.NewAccount(ctx, "apiServiceAccount", &serviceaccount.AccountArgs{
		AccountId:   pulumi.String("api-service"),
		DisplayName: pulumi.String("API Service Account"),
	},
		pulumi.Provider(prov),
	)
	if err != nil {
		return nil, err
	}

	_, err = projects.NewIAMMember(ctx, "firestoreAccess", &projects.IAMMemberArgs{
		Role: pulumi.String("roles/datastore.user"), // Firestore read/write
		Member: apiSA.Email.ApplyT(func(email string) string {
			return fmt.Sprintf("serviceAccount:%s", email)
		}).(pulumi.StringOutput),
		Project: pulumi.String(projectID),
	},
		pulumi.Provider(prov),
	)
	if err != nil {
		return nil, err
	}

	return apiSA, nil
}

func createCloudRunService(ctx *pulumi.Context,
	img *docker.Image,
	apiSA *serviceaccount.Account,
	sr *secretRefs,
	prov *gcp.Provider,
	res ...pulumi.Resource) (*cloudrun.Service, error) {
	gcpCfg := config.New(ctx, "gcp")
	crCfg := config.New(ctx, "cloudrun")
	plaidCfg := config.New(ctx, "plaid")

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

	return cloudrun.NewService(ctx, "apiService", &cloudrun.ServiceArgs{
		Location: pulumi.String(region),

		Template: &cloudrun.ServiceTemplateArgs{

			Metadata: &cloudrun.ServiceTemplateMetadataArgs{
				// ---- AUTOSCALING + INSTANCE SIZE ----
				Annotations: pulumi.StringMap{
					// Enable Identity Platform (Firebase) authentication
					"run.googleapis.com/launch-stage":      pulumi.String("BETA"),
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
							&cloudrun.ServiceTemplateSpecContainerEnvArgs{
								Name:  pulumi.String("PLAIDENVIRONMENT"),
								Value: pulumi.String(plaidEnv),
							},
							&cloudrun.ServiceTemplateSpecContainerEnvArgs{
								Name: pulumi.String("PLAIDCLIENTID"),
								ValueFrom: &cloudrun.ServiceTemplateSpecContainerEnvValueFromArgs{
									SecretKeyRef: &cloudrun.ServiceTemplateSpecContainerEnvValueFromSecretKeyRefArgs{
										Name: sr.plaidClientIDName,
										Key:  pulumi.String("latest"),
									},
								},
							},
							&cloudrun.ServiceTemplateSpecContainerEnvArgs{
								Name: pulumi.String("PLAIDSECRET"),
								ValueFrom: &cloudrun.ServiceTemplateSpecContainerEnvValueFromArgs{
									SecretKeyRef: &cloudrun.ServiceTemplateSpecContainerEnvValueFromSecretKeyRefArgs{
										Name: sr.plaidSecretName,
										Key:  pulumi.String("latest"),
									},
								},
							},
						},
					},
				},
			},
		},
	},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
}

func setIAMAccessPolicy(ctx *pulumi.Context, svc *cloudrun.Service, prov *gcp.Provider) error {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	_, err := cloudrun.NewIamMember(ctx, "denyUnauthenticated", &cloudrun.IamMemberArgs{
		Service:  svc.Name,
		Location: pulumi.String(region),
		Role:     pulumi.String("roles/run.invoker"),

		// Allow requests to reach Identity Platform (Firebase) auth
		Member: pulumi.String("allUsers"),
	},
		pulumi.Provider(prov),
	)
	return err
}

func createSecrets(ctx *pulumi.Context) (*secretRefs, error) {
	var err error
	sr := new(secretRefs)

	plaidCfg := config.New(ctx, "plaid")
	plaidClientID := plaidCfg.RequireSecret("clientId")
	plaidSecret := plaidCfg.RequireSecret("secret")

	sr.plaidClientIDName, err = secret.AddSecret(ctx, "plaidClientIdSecret", "plaidClientId", plaidClientID)
	if err != nil {
		return nil, err
	}

	sr.plaidSecretName, err = secret.AddSecret(ctx, "plaidSecretSecret", "plaidSecret", plaidSecret)
	if err != nil {
		return nil, err
	}

	return sr, nil
}
