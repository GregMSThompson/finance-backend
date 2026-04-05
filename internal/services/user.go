package services

import (
	"context"
	"strings"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
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

// SetFCMToken updates the FCM device token for a user.
func (s *userService) SetFCMToken(ctx context.Context, uid string, req dto.SetFCMTokenRequest) error {
	log := logger.FromContext(ctx)

	if req.Token == "" {
		return errs.NewValidationError("token is required")
	}

	user, err := s.Store.GetUser(ctx, uid)
	if err != nil {
		log.Error("failed to get user for fcm token update", "error", err)
		return err
	}

	user.FCMToken = req.Token
	user.UpdatedAt = time.Now()

	if err := s.Store.UpdateUser(ctx, user); err != nil {
		log.Error("failed to update fcm token", "error", err)
		return err
	}

	log.Info("fcm token updated successfully")
	return nil
}

func (s *userService) CreateUser(ctx context.Context, uid, email string, req dto.CreateUserRequest) error {
	log := logger.FromContext(ctx)

	var missing []string
	if req.FirstName == "" {
		missing = append(missing, "firstName")
	}
	if req.LastName == "" {
		missing = append(missing, "lastName")
	}
	if len(missing) > 0 {
		return errs.NewValidationError("missing required fields: " + strings.Join(missing, ", "))
	}

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
