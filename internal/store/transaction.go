package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
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

func (s *transactionStore) Query(ctx context.Context, uid string, q dto.TransactionQuery, handle func(*models.Transaction) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	txCh, errCh := s.query(ctx, uid, q)

	for txCh != nil || errCh != nil {
		select {
		case tx, ok := <-txCh:
			if !ok {
				txCh = nil
				continue
			}
			if err := handle(tx); err != nil {
				return err
			}
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *transactionStore) query(ctx context.Context, uid string, q dto.TransactionQuery) (<-chan *models.Transaction, <-chan error) {
	out := make(chan *models.Transaction, 10)
	errCh := make(chan error, 1)

	query := s.txCollection(uid).Query
	if q.Pending != nil {
		query = query.Where("pending", "==", *q.Pending)
	}
	if q.PFCPrimary != nil {
		query = query.Where("pfcPrimary", "==", *q.PFCPrimary)
	}
	if q.BankID != nil {
		query = query.Where("bankId", "==", *q.BankID)
	}
	if q.DateFrom != nil {
		query = query.Where("date", ">=", *q.DateFrom)
	}
	if q.DateTo != nil {
		query = query.Where("date", "<=", *q.DateTo)
	}

	orderField := q.OrderBy
	if orderField == "" {
		orderField = "date"
	}
	dir := firestore.Asc
	if q.Desc {
		dir = firestore.Desc
	}
	query = query.OrderBy(orderField, dir)

	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}

	iter := query.Documents(ctx)
	go func() {
		defer close(out)
		defer close(errCh)
		defer iter.Stop()

		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				errCh <- errs.NewDatabaseError("read", "failed to query transactions", err)
				return
			}

			var tx models.Transaction
			if err := doc.DataTo(&tx); err != nil {
				errCh <- errs.NewDatabaseError("read", "failed to parse transaction data", err)
				return
			}

			select {
			case out <- &tx:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
	}()

	return out, errCh
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
		job, err := bw.Set(doc, t)
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("create", "failed to upsert transaction", err)
		}
		jobs = append(jobs, job)
	}

	// Flush and close the writer, then wait on each job for errors.
	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return errs.NewDatabaseError("create", "failed to commit transaction batch", err)
		}
	}

	return nil
}

func (s *transactionStore) GetCursor(ctx context.Context, uid, bankID string) (string, error) {
	snap, err := s.cursorDoc(uid, bankID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", nil
		}
		return "", errs.NewDatabaseError("read", "failed to get cursor", err)
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
	if err != nil {
		return errs.NewDatabaseError("update", "failed to set cursor", err)
	}
	return nil
}

func (s *transactionStore) DeleteByBank(ctx context.Context, uid, bankID string) error {
	iter := s.txCollection(uid).Where("bankId", "==", bankID).Documents(ctx)
	bw := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("delete", "failed to query transactions for deletion", err)
		}
		job, err := bw.Delete(doc.Ref)
		if err != nil {
			bw.End()
			return errs.NewDatabaseError("delete", "failed to delete transaction", err)
		}
		jobs = append(jobs, job)
	}

	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return errs.NewDatabaseError("delete", "failed to commit transaction deletion batch", err)
		}
	}

	return nil
}

func (s *transactionStore) DeleteCursor(ctx context.Context, uid, bankID string) error {
	_, err := s.cursorDoc(uid, bankID).Delete(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		return errs.NewDatabaseError("delete", "failed to delete cursor", err)
	}
	return nil
}
