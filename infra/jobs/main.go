package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/job"
	"github.com/GregMSThompson/finance-backend/infra/provider"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}

		dockerManager := docker.New(prov)
		alertEvaluator := job.NewAlertEvaluator(prov, dockerManager)

		workerCfg := config.New(ctx, "worker")
		workerRef, err := pulumi.NewStackReference(ctx, "worker", &pulumi.StackReferenceArgs{
			Name: pulumi.String(workerCfg.Require("stack")),
		})
		if err != nil {
			return err
		}

		workerURL := workerRef.GetOutput(pulumi.String("workerServiceURL")).ApplyT(func(v any) string {
			if v == nil {
				return ""
			}
			return v.(string)
		}).(pulumi.StringOutput)
		workerAudience := workerRef.GetOutput(pulumi.String("workerAudience")).ApplyT(func(v any) string {
			if v == nil {
				return ""
			}
			return v.(string)
		}).(pulumi.StringOutput)
		workerServiceName := workerRef.GetOutput(pulumi.String("workerServiceName")).ApplyT(func(v any) string {
			if v == nil {
				return ""
			}
			return v.(string)
		}).(pulumi.StringOutput)

		repo, err := dockerManager.CreateAlertEvaluatorRepo(ctx)
		if err != nil {
			return err
		}

		return alertEvaluator.Deploy(ctx, workerURL, workerAudience, workerServiceName, repo)
	})
}
