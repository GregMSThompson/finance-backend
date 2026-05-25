package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
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
		worker := cloudrun.NewWorker(prov, dockerManager, secretManager)

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

		secrets := cloudrun.SecretRefs{
			PlaidClientIDName: sharedRef.GetOutput(pulumi.String("plaidClientIdSecretName")).ApplyT(func(v any) string {
				if v == nil {
					return ""
				}
				return v.(string)
			}).(pulumi.StringOutput),
			PlaidSecretName: sharedRef.GetOutput(pulumi.String("plaidSecretSecretName")).ApplyT(func(v any) string {
				if v == nil {
					return ""
				}
				return v.(string)
			}).(pulumi.StringOutput),
		}

		repo, err := dockerManager.CreateWorkerRepo(ctx)
		if err != nil {
			return err
		}

		deployment, err := worker.Deploy(ctx, keyID, secrets, repo)
		if err != nil {
			return err
		}

		ctx.Export("workerAudience", pulumi.String(deployment.Audience))
		ctx.Export("workerServiceAccountEmail", deployment.ServiceAccount.Email)
		ctx.Export("workerServiceName", deployment.Service.Name)

		return nil
	})
}
