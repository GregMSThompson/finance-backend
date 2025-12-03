package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

type UserStore interface {
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, uid string) (*models.User, error)
}

type userService struct {
	Log   *slog.Logger
	Store UserStore
}

func NewUserService(log *slog.Logger, store UserStore) *userService {
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

	s.Log.Info("User created", "id", user.UID)
	s.Log.Debug("User created", slog.Any("user", user))

	return s.Store.CreateUser(ctx, user)
}
