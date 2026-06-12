package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type kmsCipher interface {
	KmsEncrypt(ctx context.Context, plaintext string) (string, error)
	KmsDecrypt(ctx context.Context, ciphertext string) (string, error)
}

type bankStore struct {
	client *firestore.Client
	kms    kmsCipher
}

// itemIndexEntry maps a Plaid item_id to the owning (uid, bankId) so webhook
// payloads (which only carry item_id) can be routed to the right bank.
type itemIndexEntry struct {
	UID    string `firestore:"uid"`
	BankID string `firestore:"bankId"`
}

func NewBankStore(client *firestore.Client, kms kmsCipher) *bankStore {
	return &bankStore{client: client, kms: kms}
}

func (s *bankStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("banks")
}

func (s *bankStore) itemIndex() *firestore.CollectionRef {
	return s.client.Collection("itemIndex")
}

func (s *bankStore) Create(ctx context.Context, uid string, bank *models.Bank) error {
	now := time.Now()
	if bank.CreatedAt.IsZero() {
		bank.CreatedAt = now
	}
	bank.UpdatedAt = now

	toStore := *bank
	token, err := s.encryptToken(ctx, bank.PlaidPublicToken)
	if err != nil {
		return err
	}
	toStore.PlaidPublicToken = token

	_, err = s.collection(uid).Doc(bank.BankID).Set(ctx, &toStore)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create bank", err)
	}

	_, err = s.itemIndex().Doc(bank.BankID).Set(ctx, itemIndexEntry{UID: uid, BankID: bank.BankID})
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create item index", err)
	}
	return nil
}

func (s *bankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	docs, err := s.collection(uid).Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list banks", err)
	}
	banks := make([]*models.Bank, 0, len(docs))
	for _, d := range docs {
		var b models.Bank
		if err := d.DataTo(&b); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse bank data", err)
		}
		if err := s.decryptToken(ctx, &b); err != nil {
			return nil, err
		}
		banks = append(banks, &b)
	}
	return banks, nil
}

func (s *bankStore) Get(ctx context.Context, uid, bankID string) (*models.Bank, error) {
	doc, err := s.collection(uid).Doc(bankID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("bank not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get bank", err)
	}
	var b models.Bank
	if err := doc.DataTo(&b); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse bank data", err)
	}
	if err := s.decryptToken(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *bankStore) Delete(ctx context.Context, uid, bankID string) error {
	_, err := s.collection(uid).Doc(bankID).Delete(ctx)
	if err != nil {
		return errs.NewDatabaseError("delete", "failed to delete bank", err)
	}
	_, err = s.itemIndex().Doc(bankID).Delete(ctx)
	if err != nil {
		return errs.NewDatabaseError("delete", "failed to delete item index", err)
	}
	return nil
}

// FindUserIDByBankID returns the owning uid for a bank/item by its bankID
// (which is the same value as Plaid's item_id). Used by the webhook handler
// to route inbound events to the right user.
func (s *bankStore) FindUserIDByBankID(ctx context.Context, bankID string) (string, error) {
	doc, err := s.itemIndex().Doc(bankID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", errs.NewNotFoundError("item not found")
		}
		return "", errs.NewDatabaseError("read", "failed to lookup item index", err)
	}
	var entry itemIndexEntry
	if err := doc.DataTo(&entry); err != nil {
		return "", errs.NewDatabaseError("read", "failed to parse item index", err)
	}
	return entry.UID, nil
}

// SetNeedsReauth flips the bank's needsReauth flag. Used by the webhook handler
// when Plaid signals ITEM_LOGIN_REQUIRED, PENDING_EXPIRATION, or LOGIN_REPAIRED.
func (s *bankStore) SetNeedsReauth(ctx context.Context, uid, bankID string, needs bool) error {
	_, err := s.collection(uid).Doc(bankID).Update(ctx, []firestore.Update{
		{Path: "needsReauth", Value: needs},
		{Path: "updatedAt", Value: time.Now()},
	})
	if err != nil {
		return errs.NewDatabaseError("update", "failed to update needsReauth", err)
	}
	return nil
}

func (s *bankStore) encryptToken(ctx context.Context, token string) (string, error) {
	if token == "" || s.kms == nil {
		return token, nil
	}
	ciphertext, err := s.kms.KmsEncrypt(ctx, token)
	if err != nil {
		return "", errs.NewEncryptionError("failed to encrypt token", err)
	}
	return ciphertext, nil
}

func (s *bankStore) decryptToken(ctx context.Context, bank *models.Bank) error {
	if bank.PlaidPublicToken == "" || s.kms == nil {
		return nil
	}
	plaintext, err := s.kms.KmsDecrypt(ctx, bank.PlaidPublicToken)
	if err != nil {
		return errs.NewEncryptionError("failed to decrypt token", err)
	}
	bank.PlaidPublicToken = plaintext
	return nil
}
