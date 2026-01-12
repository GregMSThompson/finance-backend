package crypto

import (
	"context"
	"encoding/base64"

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

// KmsEncrypt encrypts plaintext using the configured KMS key name and returns base64 text.
func (k *kms) KmsEncrypt(ctx context.Context, plaintext string) (string, error) {
	resp, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      k.keyName,
		Plaintext: []byte(plaintext),
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(resp.Ciphertext), nil
}

// KmsDecrypt decrypts base64 ciphertext using the configured KMS key name.
func (k *kms) KmsDecrypt(ctx context.Context, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	resp, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       k.keyName,
		Ciphertext: raw,
	})
	if err != nil {
		return "", err
	}
	return string(resp.Plaintext), nil
}
