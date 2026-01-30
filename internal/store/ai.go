package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type aiStore struct {
	client *firestore.Client
}

func NewAIStore(client *firestore.Client) *aiStore {
	return &aiStore{client: client}
}

func (s *aiStore) messagesCollection(uid, sessionID string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(uid).Collection("ai_sessions").Doc(sessionID).Collection("messages")
}

func (s *aiStore) SaveMessage(ctx context.Context, uid, sessionID string, msg dto.AIMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	_, _, err := s.messagesCollection(uid, sessionID).Add(ctx, msg)
	return err
}

func (s *aiStore) ListMessages(ctx context.Context, uid, sessionID string, limit int) ([]dto.AIMessage, error) {
	query := s.messagesCollection(uid, sessionID).Query.OrderBy("createdAt", firestore.Desc)
	if limit > 0 {
		query = query.Limit(limit)
	}

	iter := query.Documents(ctx)
	defer iter.Stop()

	var out []dto.AIMessage
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var msg dto.AIMessage
		if err := doc.DataTo(&msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}

	reverseMessages(out)
	return out, nil
}

func reverseMessages(msgs []dto.AIMessage) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}
