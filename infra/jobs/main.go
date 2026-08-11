package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	infracloudtasks "github.com/GregMSThompson/finance-backend/infra/cloudtasks"
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
		goalEvaluator := job.NewGoalEvaluator(prov, dockerManager)

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

		alertRepo, err := dockerManager.CreateAlertEvaluatorRepo(ctx)
		if err != nil {
			return err
		}

		goalRepo, err := dockerManager.CreateGoalEvaluatorRepo(ctx)
		if err != nil {
			return err
		}

		// The alert and goal evaluators share one notification-delivery queue,
		// created once here and passed into both deployments.
		queue, err := infracloudtasks.CreateQueue(ctx, prov, "notificationDeliveryQueue", infracloudtasks.NotificationDeliveryQueueName)
		if err != nil {
			return err
		}
		ctx.Export("notificationDeliveryQueueName", queue.Name)

		if err := alertEvaluator.Deploy(ctx, queue, workerURL, workerAudience, workerServiceName, alertRepo); err != nil {
			return err
		}

		return goalEvaluator.Deploy(ctx, queue, workerURL, workerAudience, workerServiceName, goalRepo)
	})
}
