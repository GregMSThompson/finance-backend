package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type stubUserStore struct {
	user            *models.User
	createUserCalls int
	err             error
}

func (s *stubUserStore) CreateUser(_ context.Context, user *models.User) error {
	s.user = user
	s.createUserCalls++
	return s.err
}

func (s *stubUserStore) UpdateUser(_ context.Context, _ *models.User) error { return nil }
func (s *stubUserStore) GetUser(_ context.Context, _ string) (*models.User, error) {
	return nil, nil
}

func newTestLogger() *slog.Logger {
	return logger.New("", logger.NewTestHandler)
}

func TestUserServiceCreateUser(t *testing.T) {
	store := &stubUserStore{}
	svc := NewUserService(store)

	ctx := helpers.TestCtx()
	now := time.Now()

	err := svc.CreateUser(ctx, "uid-123", "user@example.com", "Jane", "Doe")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if store.createUserCalls != 1 {
		t.Fatalf("CreateUser called %d times, want 1", store.createUserCalls)
	}

	if store.user == nil {
		t.Fatalf("store received nil user")
	}

	if store.user.UID != "uid-123" || store.user.Email != "user@example.com" {
		t.Fatalf("unexpected user identifiers: %+v", store.user)
	}

	if store.user.FirstName != "Jane" || store.user.LastName != "Doe" {
		t.Fatalf("unexpected user name: %+v", store.user)
	}

	if store.user.CreatedAt.IsZero() || store.user.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not set: %+v", store.user)
	}

	if store.user.CreatedAt.Before(now) {
		t.Fatalf("CreatedAt set earlier than call time: %v before %v", store.user.CreatedAt, now)
	}
}

func TestUserServiceCreateUserStoreError(t *testing.T) {
	store := &stubUserStore{err: errors.New("store failure")}
	svc := NewUserService(store)

	ctx := helpers.TestCtx()
	err := svc.CreateUser(ctx, "uid-456", "user2@example.com", "John", "Smith")

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if store.createUserCalls != 1 {
		t.Fatalf("CreateUser called %d times, want 1", store.createUserCalls)
	}

	if store.user == nil || store.user.UID != "uid-456" {
		t.Fatalf("store did not receive expected user payload: %+v", store.user)
	}
}
