package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/firestore"
	"github.com/GregMSThompson/finance-backend/infra/identity"
	"github.com/GregMSThompson/finance-backend/infra/kms"
	"github.com/GregMSThompson/finance-backend/infra/provider"
	"github.com/GregMSThompson/finance-backend/infra/secret"
	"github.com/GregMSThompson/finance-backend/infra/vertex"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// set default provider with the correct project
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}
		kmsManager := kms.New(prov)
		dockerManager := docker.New(prov)
		secretManager := secret.New(prov)
		api := cloudrun.NewAPI(prov, dockerManager, secretManager)

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

		// enable vertex service for gemini queries
		err = vertex.SetupVertex(ctx, prov)
		if err != nil {
			return err
		}

		artifactRegistry, err := docker.SetupArtifactRegistry(ctx, prov)
		if err != nil {
			return err
		}

		// enable kms service
		ks, err := kms.SetupKMS(ctx, prov)
		if err != nil {
			return err
		}

		keyID, err := kmsManager.CreateKey(ctx, "app-keys", "user", ks)
		if err != nil {
			return err
		}

		// create docker repo
		repo, err := dockerManager.CreateAPIRepo(ctx, artifactRegistry)
		if err != nil {
			return err
		}

		_, err = api.Deploy(ctx, keyID, ident, repo, ks, sm)
		if err != nil {
			return err
		}

		return nil
	})
}
