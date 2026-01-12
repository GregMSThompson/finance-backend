package kms

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/kms"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

var (
	service *projects.Service
)

func SetupKMS(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	var err error
	service, err = projects.NewService(ctx, "kmsService", &projects.ServiceArgs{
		Service: pulumi.String("cloudkms.googleapis.com"),
	}, pulumi.Provider(prov))
	if err != nil {
		return nil, err
	}

	return service, nil
}

// CreateKey creates a key ring + crypto key and returns the crypto key ID.
func CreateKey(
	ctx *pulumi.Context,
	prov *gcp.Provider,
	keyRingID string,
	keyID string,
) (pulumi.StringOutput, error) {
	gcpCfg := config.New(ctx, "gcp")
	location := gcpCfg.Require("region")

	ring, err := kms.NewKeyRing(ctx, fmt.Sprintf("%s-ring", keyRingID), &kms.KeyRingArgs{
		Location: pulumi.String(location),
		Name:     pulumi.String(keyRingID),
	},
		pulumi.Provider(prov),
		pulumi.DependsOn([]pulumi.Resource{service}),
	)
	if err != nil {
		return pulumi.String("").ToStringOutput(), err
	}

	key, err := kms.NewCryptoKey(ctx, fmt.Sprintf("%s-key", keyID), &kms.CryptoKeyArgs{
		KeyRing:        ring.ID(),
		Name:           pulumi.String(keyID),
		Purpose:        pulumi.String("ENCRYPT_DECRYPT"),
		RotationPeriod: pulumi.String("7776000s"), // 90 days
	},
		pulumi.Provider(prov),
	)
	if err != nil {
		return pulumi.String("").ToStringOutput(), err
	}

	return key.ID().ToStringOutput(), nil
}
