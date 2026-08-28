package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type stubUserStore struct {
	user            *models.User
	createUserCalls int
	createUserErr   error

	getUser    *models.User
	getUserErr error

	updatedUser     *models.User
	updateUserCalls int
	updateUserErr   error
}

func (s *stubUserStore) CreateUser(_ context.Context, user *models.User) error {
	s.user = user
	s.createUserCalls++
	return s.createUserErr
}

func (s *stubUserStore) UpdateUser(_ context.Context, user *models.User) error {
	s.updatedUser = user
	s.updateUserCalls++
	return s.updateUserErr
}

func (s *stubUserStore) GetUser(_ context.Context, _ string) (*models.User, error) {
	return s.getUser, s.getUserErr
}

func newTestLogger() *slog.Logger {
	return logger.New("", logger.NewTestHandler)
}

func TestUserServiceCreateUser(t *testing.T) {
	store := &stubUserStore{}
	svc := NewUserService(store)

	ctx := helpers.TestCtx()
	now := time.Now()

	err := svc.CreateUser(ctx, "uid-123", "user@example.com", dto.CreateUserRequest{FirstName: "Jane", LastName: "Doe"})
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
	store := &stubUserStore{createUserErr: errors.New("store failure")}
	svc := NewUserService(store)

	ctx := helpers.TestCtx()
	err := svc.CreateUser(ctx, "uid-456", "user2@example.com", dto.CreateUserRequest{FirstName: "John", LastName: "Smith"})

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

func TestUserServiceCreateUserMissingFields(t *testing.T) {
	tests := []struct {
		name        string
		req         dto.CreateUserRequest
		wantMissing []string
	}{
		{
			name:        "missing firstName",
			req:         dto.CreateUserRequest{LastName: "Doe"},
			wantMissing: []string{"firstName"},
		},
		{
			name:        "missing lastName",
			req:         dto.CreateUserRequest{FirstName: "Jane"},
			wantMissing: []string{"lastName"},
		},
		{
			name:        "missing both",
			req:         dto.CreateUserRequest{},
			wantMissing: []string{"firstName", "lastName"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &stubUserStore{}
			svc := NewUserService(store)

			err := svc.CreateUser(helpers.TestCtx(), "uid-123", "user@example.com", tc.req)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			for _, field := range tc.wantMissing {
				if !strings.Contains(err.Error(), field) {
					t.Fatalf("error %q does not mention missing field %q", err.Error(), field)
				}
			}
			if store.createUserCalls != 0 {
				t.Fatalf("store should not be called on validation error")
			}
		})
	}
}

func TestUserServiceSetFCMToken(t *testing.T) {
	existing := &models.User{
		UID:      "uid-123",
		Email:    "user@example.com",
		FCMToken: "old-token",
	}
	store := &stubUserStore{getUser: existing}
	svc := NewUserService(store)

	err := svc.SetFCMToken(helpers.TestCtx(), "uid-123", dto.SetFCMTokenRequest{Token: "new-token"})
	if err != nil {
		t.Fatalf("SetFCMToken returned error: %v", err)
	}

	if store.updateUserCalls != 1 {
		t.Fatalf("UpdateUser called %d times, want 1", store.updateUserCalls)
	}
	if store.updatedUser.FCMToken != "new-token" {
		t.Fatalf("unexpected FCMToken on updated user: %s", store.updatedUser.FCMToken)
	}
	if store.updatedUser.UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt was not set")
	}
}

func TestUserServiceSetFCMTokenEmptyToken(t *testing.T) {
	store := &stubUserStore{}
	svc := NewUserService(store)

	err := svc.SetFCMToken(helpers.TestCtx(), "uid-123", dto.SetFCMTokenRequest{Token: ""})
	if err == nil {
		t.Fatalf("expected validation error for empty token, got nil")
	}
	if store.updateUserCalls != 0 {
		t.Fatalf("store should not be called on validation error")
	}
}

func TestUserServiceSetFCMTokenGetUserError(t *testing.T) {
	store := &stubUserStore{getUserErr: errors.New("store failure")}
	svc := NewUserService(store)

	err := svc.SetFCMToken(helpers.TestCtx(), "uid-123", dto.SetFCMTokenRequest{Token: "new-token"})
	if err == nil {
		t.Fatalf("expected error from GetUser, got nil")
	}
	if store.updateUserCalls != 0 {
		t.Fatalf("UpdateUser should not be called when GetUser fails")
	}
}

func TestUserServiceSetFCMTokenUpdateUserError(t *testing.T) {
	existing := &models.User{UID: "uid-123"}
	store := &stubUserStore{
		getUser:       existing,
		updateUserErr: errors.New("store failure"),
	}
	svc := NewUserService(store)

	err := svc.SetFCMToken(helpers.TestCtx(), "uid-123", dto.SetFCMTokenRequest{Token: "new-token"})
	if err == nil {
		t.Fatalf("expected error from UpdateUser, got nil")
	}
}
