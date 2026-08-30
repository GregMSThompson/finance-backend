package store

import (
	"context"

	"cloud.google.com/go/firestore"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
)

type orphanedBankStore struct {
	client *firestore.Client
	kms    kmsCipher
}

func NewOrphanedBankStore(client *firestore.Client, kms kmsCipher) *orphanedBankStore {
	return &orphanedBankStore{client: client, kms: kms}
}

func (s *orphanedBankStore) collection() *firestore.CollectionRef {
	return s.client.Collection("orphanedBanks")
}

// Record persists an Item we couldn't remove, keyed by BankID so a retry
// overwrites the prior attempt rather than duplicating it. The access token is
// KMS-encrypted at rest, mirroring the bank store.
func (s *orphanedBankStore) Record(ctx context.Context, item *models.OrphanedBank) error {
	toStore := *item
	if toStore.CreatedAt.IsZero() {
		toStore.CreatedAt = clock.Now(ctx)
	}

	if toStore.AccessToken != "" && s.kms != nil {
		ciphertext, err := s.kms.KmsEncrypt(ctx, toStore.AccessToken)
		if err != nil {
			return errs.NewEncryptionError("failed to encrypt token", err)
		}
		toStore.AccessToken = ciphertext
	}

	if _, err := s.collection().Doc(item.BankID).Set(ctx, &toStore); err != nil {
		return errs.NewDatabaseError("create", "failed to record orphaned plaid item", err)
	}
	return nil
}
