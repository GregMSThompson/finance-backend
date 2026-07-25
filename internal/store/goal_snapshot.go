package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type goalSnapshotStore struct {
	client *firestore.Client
}

func NewGoalSnapshotStore(client *firestore.Client) *goalSnapshotStore {
	return &goalSnapshotStore{client: client}
}

func (s *goalSnapshotStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("goalSnapshots")
}

func (s *goalSnapshotStore) Create(ctx context.Context, uid string, snap *models.GoalSnapshot) error {
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now()
	}
	_, err := s.collection(uid).Doc(snap.SnapshotID).Set(ctx, snap)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create goal snapshot", err)
	}
	return nil
}

// Latest returns the most recent snapshot for a goal, or (nil, nil) if the goal
// has not been evaluated yet.
func (s *goalSnapshotStore) Latest(ctx context.Context, uid, goalID string) (*models.GoalSnapshot, error) {
	iter := s.collection(uid).
		Where("goalId", "==", goalID).
		OrderBy("createdAt", firestore.Desc).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to get latest goal snapshot", err)
	}
	var snap models.GoalSnapshot
	if err := doc.DataTo(&snap); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse goal snapshot data", err)
	}
	return &snap, nil
}

// ListForGoal returns up to limit of a goal's snapshots, most recent first.
func (s *goalSnapshotStore) ListForGoal(ctx context.Context, uid, goalID string, limit int) ([]*models.GoalSnapshot, error) {
	q := s.collection(uid).
		Where("goalId", "==", goalID).
		OrderBy("createdAt", firestore.Desc)
	if limit > 0 {
		q = q.Limit(limit)
	}
	docs, err := q.Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list goal snapshots", err)
	}
	snaps := make([]*models.GoalSnapshot, 0, len(docs))
	for _, d := range docs {
		var snap models.GoalSnapshot
		if err := d.DataTo(&snap); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse goal snapshot data", err)
		}
		snaps = append(snaps, &snap)
	}
	return snaps, nil
}
