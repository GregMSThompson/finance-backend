package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type userUSStore interface {
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, uid string) (*models.User, error)
}

type userService struct {
	Log   *slog.Logger
	Store userUSStore
}

func NewUserService(log *slog.Logger, store userUSStore) *userService {
	return &userService{
		Log:   log,
		Store: store,
	}
}

func (s *userService) CreateUser(ctx context.Context, uid, email, first, last string) error {
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
		return err
	}

	s.Log.Info("User created", "id", user.UID)
	s.Log.Debug("User created", slog.Any("user", user))
	return nil
}
