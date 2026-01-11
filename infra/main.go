package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/firestore"
	"github.com/GregMSThompson/finance-backend/infra/identity"
	"github.com/GregMSThompson/finance-backend/infra/provider"
	"github.com/GregMSThompson/finance-backend/infra/secret"
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

		// enable the secrets manager service
		sm, err := secret.SetupSecretManager(ctx, prov)
		if err != nil {
			return err
		}

		// enable firestore and create a database for the project
		err = firestore.SetupFirestore(ctx, prov)
		if err != nil {
			return err
		}

		// create docker repo
		repo, err := docker.CreateCloudrunRepo(ctx, prov)
		if err != nil {
			return err
		}

		_, err = cloudrun.SetupCloudRun(ctx, prov, ident, repo, sm)
		if err != nil {
			return err
		}

		return nil
	})
}
