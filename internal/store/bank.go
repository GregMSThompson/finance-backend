package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
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

func NewBankStore(client *firestore.Client, kms kmsCipher) *bankStore {
	return &bankStore{client: client, kms: kms}
}

func (s *bankStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("banks")
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
	return err
}

func (s *bankStore) List(ctx context.Context, uid string) ([]*models.Bank, error) {
	docs, err := s.collection(uid).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	banks := make([]*models.Bank, 0, len(docs))
	for _, d := range docs {
		var b models.Bank
		if err := d.DataTo(&b); err != nil {
			return nil, err
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
		return nil, err
	}
	var b models.Bank
	if err := doc.DataTo(&b); err != nil {
		return nil, err
	}
	if err := s.decryptToken(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *bankStore) Delete(ctx context.Context, uid, bankID string) error {
	_, err := s.collection(uid).Doc(bankID).Delete(ctx)
	return err
}

func (s *bankStore) encryptToken(ctx context.Context, token string) (string, error) {
	if token == "" || s.kms == nil {
		return token, nil
	}
	ciphertext, err := s.kms.KmsEncrypt(ctx, token)
	if err != nil {
		return "", err
	}
	return ciphertext, nil
}

func (s *bankStore) decryptToken(ctx context.Context, bank *models.Bank) error {
	if bank.PlaidPublicToken == "" || s.kms == nil {
		return nil
	}
	plaintext, err := s.kms.KmsDecrypt(ctx, bank.PlaidPublicToken)
	if err != nil {
		return err
	}
	bank.PlaidPublicToken = plaintext
	return nil
}
