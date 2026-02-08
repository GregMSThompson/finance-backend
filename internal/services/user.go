package services

import (
	"context"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type userUSStore interface {
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, uid string) (*models.User, error)
}

type userService struct {
	Store userUSStore
}

func NewUserService(store userUSStore) *userService {
	return &userService{
		Store: store,
	}
}

func (s *userService) CreateUser(ctx context.Context, uid, email, first, last string) error {
	// Get logger from context - already has uid, email, request_id, method, path
	log := logger.FromContext(ctx)

	user := &models.User{
		UID:       uid,
		Email:     email,
		FirstName: first,
		LastName:  last,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.Store.CreateUser(ctx, user)
	if err != nil {
		log.Error("failed to create user in store", "error", err)
		return err
	}

	// uid and email are automatically included from context
	log.Info("user created successfully", "first_name", first, "last_name", last)
	log.Debug("user created with full details", "user", user)

	return nil
}
