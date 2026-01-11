package store

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Secrets path
// projects/{project}/secrets/plaid/access-token/{uid}/{itemID}/versions/{version}

type plaidSecretsStore struct {
	client    *secretmanager.Client
	projectID string
	prefix    string
}

func NewPlaidSecretsStore(client *secretmanager.Client, projectID string) *plaidSecretsStore {
	return &plaidSecretsStore{
		client:    client,
		projectID: projectID,
		prefix:    "plaid-access-token",
	}
}

func (s *plaidSecretsStore) secretID(uid, itemID string) string {
	return fmt.Sprintf("%s-%s-%s", s.prefix, uid, itemID)
}

func (s *plaidSecretsStore) secretName(uid, itemID string) string {
	return fmt.Sprintf("projects/%s/secrets/%s", s.projectID, s.secretID(uid, itemID))
}

func (s *plaidSecretsStore) ensureSecret(ctx context.Context, uid, itemID string) error {
	name := s.secretName(uid, itemID)
	_, err := s.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if status.Code(err) == codes.NotFound {
		_, err = s.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   fmt.Sprintf("projects/%s", s.projectID),
			SecretId: s.secretID(uid, itemID),
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{Automatic: &secretmanagerpb.Replication_Automatic{}},
				},
			},
		})
	}
	return err
}

func (s *plaidSecretsStore) StorePlaidToken(ctx context.Context, uid, itemID, token string) error {
	if err := s.ensureSecret(ctx, uid, itemID); err != nil {
		return err
	}
	_, err := s.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: s.secretName(uid, itemID),
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(token),
		},
	})
	return err
}

func (s *plaidSecretsStore) GetPlaidToken(ctx context.Context, uid, itemID string) (string, error) {
	res, err := s.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", s.secretName(uid, itemID)),
	})
	if err != nil {
		return "", err
	}
	return string(res.Payload.Data), nil
}

func (s *plaidSecretsStore) DeletePlaidToken(ctx context.Context, uid, itemID string) error {
	err := s.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: s.secretName(uid, itemID),
	})
	return err
}
