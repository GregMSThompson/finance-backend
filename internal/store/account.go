package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type accountStore struct {
	client *firestore.Client
}

func NewAccountStore(client *firestore.Client) *accountStore {
	return &accountStore{client: client}
}

func (s *accountStore) collection(uid, bankID string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("banks").Doc(bankID).Collection("accounts")
}

// UpsertBatch writes all accounts for the given bank in a single bulk operation.
// Existing documents are overwritten; safe to call on re-link because Plaid
// account_ids are stable for the lifetime of an Item.
func (s *accountStore) UpsertBatch(ctx context.Context, uid, bankID string, accounts []models.Account) error {
	if len(accounts) == 0 {
		return nil
	}

	bw := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(accounts))
	now := time.Now()

	for _, a := range accounts {
		a.UpdatedAt = now
		if a.CreatedAt.IsZero() {
			a.CreatedAt = now
		}
		doc := s.collection(uid, bankID).Doc(a.AccountID)
		job, err := bw.Set(doc, a)
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("create", "failed to upsert account", err)
		}
		jobs = append(jobs, job)
	}

	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return errs.NewDatabaseError("create", "failed to commit account batch", err)
		}
	}

	return nil
}

// List returns all accounts stored under the given bank subcollection.
func (s *accountStore) List(ctx context.Context, uid, bankID string) ([]models.Account, error) {
	iter := s.collection(uid, bankID).Documents(ctx)
	var accounts []models.Account

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errs.NewDatabaseError("read", "failed to list accounts", err)
		}
		var a models.Account
		if err := doc.DataTo(&a); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to decode account", err)
		}
		accounts = append(accounts, a)
	}

	return accounts, nil
}

// DeleteByBank removes all accounts under the given bank subcollection. Called
// as part of the bank delete cascade so accounts don't outlive their parent Item.
func (s *accountStore) DeleteByBank(ctx context.Context, uid, bankID string) error {
	iter := s.collection(uid, bankID).Documents(ctx)
	bw := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("delete", "failed to query accounts for deletion", err)
		}
		job, err := bw.Delete(doc.Ref)
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("delete", "failed to delete account", err)
		}
		jobs = append(jobs, job)
	}

	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return errs.NewDatabaseError("delete", "failed to commit account deletion batch", err)
		}
	}

	return nil
}
