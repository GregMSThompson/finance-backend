package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/cloudtasks"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/provider"
	"github.com/GregMSThompson/finance-backend/infra/secret"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}
		dockerManager := docker.New(prov)
		secretManager := secret.New(prov)
		api := cloudrun.NewAPI(prov, dockerManager, secretManager)

		sharedCfg := config.New(ctx, "shared")
		sharedRef, err := pulumi.NewStackReference(ctx, "shared", &pulumi.StackReferenceArgs{
			Name: pulumi.String(sharedCfg.Require("stack")),
		})
		if err != nil {
			return err
		}

		keyID := sharedRef.GetOutput(pulumi.String("kmsKeyId")).ApplyT(func(v any) string {
			if v == nil {
				return ""
			}
			return v.(string)
		}).(pulumi.StringOutput)

		workerCfg := config.New(ctx, "worker")
		workerRef, err := pulumi.NewStackReference(ctx, "worker", &pulumi.StackReferenceArgs{
			Name: pulumi.String(workerCfg.Require("stack")),
		})
		if err != nil {
			return err
		}

		worker := cloudrun.WorkerRefs{
			ServiceURL:  workerRef.GetOutput(pulumi.String("workerServiceURL")).ApplyT(func(v any) string {
				if v == nil {
					return ""
				}
				return v.(string)
			}).(pulumi.StringOutput),
			Audience: workerRef.GetOutput(pulumi.String("workerAudience")).ApplyT(func(v any) string {
				if v == nil {
					return ""
				}
				return v.(string)
			}).(pulumi.StringOutput),
			ServiceName: workerRef.GetOutput(pulumi.String("workerServiceName")).ApplyT(func(v any) string {
				if v == nil {
					return ""
				}
				return v.(string)
			}).(pulumi.StringOutput),
		}

		jobsQueue, err := cloudtasks.CreateQueue(ctx, prov, "jobsQueue", "jobs")
		if err != nil {
			return err
		}

		repo, err := dockerManager.CreateAPIRepo(ctx)
		if err != nil {
			return err
		}

		apiSA, err := api.Deploy(ctx, keyID, jobsQueue, worker, repo)
		if err != nil {
			return err
		}

		ctx.Export("apiServiceAccountEmail", apiSA.Email)
		ctx.Export("jobsQueueName", jobsQueue.Name)

		return nil
	})
}
