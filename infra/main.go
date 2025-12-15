package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/GregMSThompson/finance-backend/infra/cloudrun"
	"github.com/GregMSThompson/finance-backend/infra/firestore"
	"github.com/GregMSThompson/finance-backend/infra/identity"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// enable identity service to allow using firebase
		_, err := identity.SetupIdentity(ctx)
		if err != nil {
			return err
		}

		// enable firestore and create a database for the project
		err = firestore.SetupFirestore(ctx)
		if err != nil {
			return err
		}

		_, err = cloudrun.SetupCloudRun(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}
