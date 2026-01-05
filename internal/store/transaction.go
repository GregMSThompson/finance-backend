package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type transactionStore struct {
	client *firestore.Client
}

func NewTransactionStore(client *firestore.Client) *transactionStore {
	return &transactionStore{client: client}
}

func (s *transactionStore) txCollection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("transactions")
}

func (s *transactionStore) cursorDoc(uid, bankID string) *firestore.DocumentRef {
	return s.client.Collection("users").Doc(uid).Collection("plaid_cursors").Doc(bankID)
}

func (s *transactionStore) UpsertBatch(ctx context.Context, uid string, txs []models.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	bw := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(txs))
	now := time.Now()

	for _, t := range txs {
		t.UpdatedAt = now
		if t.CreatedAt.IsZero() {
			t.CreatedAt = now
		}

		doc := s.txCollection(uid).Doc(t.TransactionID)
		job, err := bw.Set(doc, t, firestore.MergeAll)
		if err != nil {
			bw.End()
			return err
		}
		jobs = append(jobs, job)
	}

	// Flush and close the writer, then wait on each job for errors.
	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return err
		}
	}

	return nil
}

func (s *transactionStore) GetCursor(ctx context.Context, uid, bankID string) (string, error) {
	snap, err := s.cursorDoc(uid, bankID).Get(ctx)
	if err != nil {
		return "", err
	}
	cursor, ok := snap.Data()["cursor"].(string)
	if !ok {
		return "", nil
	}
	return cursor, nil
}

func (s *transactionStore) SetCursor(ctx context.Context, uid, bankID, cursor string) error {
	_, err := s.cursorDoc(uid, bankID).Set(ctx, map[string]interface{}{
		"cursor":    cursor,
		"updatedAt": time.Now(),
	}, firestore.MergeAll)
	return err
}
