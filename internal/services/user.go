package services

import (
	"context"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
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

func (s *userService) CreateUser(ctx context.Context, uid, email string, req dto.CreateUserRequest) error {
	log := logger.FromContext(ctx)

	user := &models.User{
		UID:       uid,
		Email:     email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.Store.CreateUser(ctx, user)
	if err != nil {
		log.Error("failed to create user in store", "error", err)
		return err
	}

	log.Info("user created successfully", "first_name", req.FirstName, "last_name", req.LastName)
	log.Debug("user created with full details", "user", user)

	return nil
}
