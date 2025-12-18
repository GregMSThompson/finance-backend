package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/firestore"
	"github.com/GregMSThompson/finance-backend/infra/identity"
	"github.com/GregMSThompson/finance-backend/infra/provider"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// set default provider with the correct project
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}

		// enable identity service to allow using firebase
		ident, err := identity.SetupIdentity(ctx, prov)
		if err != nil {
			return err
		}

		// enable firestore and create a database for the project
		err = firestore.SetupFirestore(ctx)
		if err != nil {
			return err
		}

		// create docker repo
		repo, err := docker.CreateCloudrunRepo(ctx)
		if err != nil {
			return err
		}

		_, err = cloudrun.SetupCloudRun(ctx, ident, repo)
		if err != nil {
			return err
		}

		return nil
	})
}
