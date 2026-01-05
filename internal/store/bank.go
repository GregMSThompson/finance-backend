package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type bankStore struct {
	client *firestore.Client
}

func NewBankStore(client *firestore.Client) *bankStore {
	return &bankStore{client: client}
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
	_, err := s.collection(uid).Doc(bank.BankID).Set(ctx, bank)
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
	return &b, nil
}

func (s *bankStore) Delete(ctx context.Context, uid, bankID string) error {
	_, err := s.collection(uid).Doc(bankID).Delete(ctx)
	return err
}
