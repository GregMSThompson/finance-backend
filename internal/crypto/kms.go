package crypto

import (
	"context"

	gcpkms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

type kms struct {
	client  *gcpkms.KeyManagementClient
	keyName string
}

func NewKMS(client *gcpkms.KeyManagementClient, keyName string) *kms {
	return &kms{client: client, keyName: keyName}
}

// KmsEncrypt encrypts plaintext using the configured KMS key name.
func (k *kms) KmsEncrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	resp, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      k.keyName,
		Plaintext: plaintext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Ciphertext, nil
}

// KmsDecrypt decrypts ciphertext using the configured KMS key name.
func (k *kms) KmsDecrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	resp, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       k.keyName,
		Ciphertext: ciphertext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Plaintext, nil
}
