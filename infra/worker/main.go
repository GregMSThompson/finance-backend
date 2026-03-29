package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/provider"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}

		dockerManager := docker.New(prov)
		worker := cloudrun.NewWorker(prov, dockerManager)

		repo, err := dockerManager.CreateWorkerRepo(ctx)
		if err != nil {
			return err
		}

		workerSA, err := worker.Deploy(ctx, repo)
		if err != nil {
			return err
		}

		ctx.Export("workerServiceAccountEmail", workerSA.Email)

		return nil
	})
}
