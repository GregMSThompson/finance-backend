package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/cloudscheduler"
	"github.com/GregMSThompson/finance-backend/infra/cloudtasks"
	"github.com/GregMSThompson/finance-backend/infra/compute"
	"github.com/GregMSThompson/finance-backend/infra/docker"
	"github.com/GregMSThompson/finance-backend/infra/fcm"
	"github.com/GregMSThompson/finance-backend/infra/firestore"
	"github.com/GregMSThompson/finance-backend/infra/identity"
	"github.com/GregMSThompson/finance-backend/infra/kms"
	"github.com/GregMSThompson/finance-backend/infra/provider"
	"github.com/GregMSThompson/finance-backend/infra/secret"
	"github.com/GregMSThompson/finance-backend/infra/vertex"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := provider.SetupDefaultProvider(ctx)
		if err != nil {
			return err
		}
		kmsManager := kms.New(prov)

		ident, err := identity.SetupIdentity(ctx, prov)
		if err != nil {
			return err
		}

		if _, err := secret.SetupSecretManager(ctx, prov); err != nil {
			return err
		}

		if err := firestore.SetupFirestore(ctx, prov); err != nil {
			return err
		}

		if err := vertex.SetupVertex(ctx, prov); err != nil {
			return err
		}

		if _, err := docker.SetupArtifactRegistry(ctx, prov); err != nil {
			return err
		}

		if _, err := cloudrun.SetupCloudRun(ctx, prov); err != nil {
			return err
		}

		if _, err := cloudtasks.SetupCloudTasks(ctx, prov); err != nil {
			return err
		}

		if _, err := cloudscheduler.SetupCloudScheduler(ctx, prov); err != nil {
			return err
		}

		if _, err := compute.SetupCompute(ctx, prov); err != nil {
			return err
		}

		if _, err := fcm.SetupFCM(ctx, prov); err != nil {
			return err
		}

		kmsService, err := kms.SetupKMS(ctx, prov)
		if err != nil {
			return err
		}

		keyID, err := kmsManager.CreateKey(ctx, "app-keys", "shared", kmsService)
		if err != nil {
			return err
		}

		ctx.Export("identityPlatformName", ident.Name)
		ctx.Export("kmsKeyId", keyID)

		return nil
	})
}
