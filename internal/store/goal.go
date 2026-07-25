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

type goalStore struct {
	client *firestore.Client
}

func NewGoalStore(client *firestore.Client) *goalStore {
	return &goalStore{client: client}
}

func (s *goalStore) collection(uid string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("goals")
}

func (s *goalStore) Create(ctx context.Context, uid string, g *models.Goal) error {
	now := time.Now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	_, err := s.collection(uid).Doc(g.GoalID).Set(ctx, g)
	if err != nil {
		return errs.NewDatabaseError("create", "failed to create goal", err)
	}
	return nil
}

func (s *goalStore) Get(ctx context.Context, uid, goalID string) (*models.Goal, error) {
	doc, err := s.collection(uid).Doc(goalID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("goal not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get goal", err)
	}
	var g models.Goal
	if err := doc.DataTo(&g); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse goal data", err)
	}
	return &g, nil
}

// List returns the user's goals filtered by status. Pass no statuses to return
// all goals; pass one or more to filter — e.g. active for the batch evaluator,
// active+paused for the Goals tab, completed+failed for past goals.
func (s *goalStore) List(ctx context.Context, uid string, statuses ...models.GoalStatus) ([]*models.Goal, error) {
	q := s.collection(uid).Query
	switch len(statuses) {
	case 0:
		// no filter — all goals
	case 1:
		q = q.Where("status", "==", string(statuses[0]))
	default:
		vals := make([]string, len(statuses))
		for i, st := range statuses {
			vals[i] = string(st)
		}
		q = q.Where("status", "in", vals)
	}
	docs, err := q.Documents(ctx).GetAll()
	if err != nil {
		return nil, errs.NewDatabaseError("read", "failed to list goals", err)
	}
	goals := make([]*models.Goal, 0, len(docs))
	for _, d := range docs {
		var g models.Goal
		if err := d.DataTo(&g); err != nil {
			return nil, errs.NewDatabaseError("read", "failed to parse goal data", err)
		}
		goals = append(goals, &g)
	}
	return goals, nil
}

func (s *goalStore) Update(ctx context.Context, uid string, g *models.Goal) error {
	g.UpdatedAt = time.Now()
	_, err := s.collection(uid).Doc(g.GoalID).Set(ctx, g)
	if err != nil {
		return errs.NewDatabaseError("update", "failed to update goal", err)
	}
	return nil
}

func (s *goalStore) Delete(ctx context.Context, uid, goalID string) error {
	_, err := s.collection(uid).Doc(goalID).Delete(ctx)
	if err != nil {
		return errs.NewDatabaseError("delete", "failed to delete goal", err)
	}
	return nil
}
