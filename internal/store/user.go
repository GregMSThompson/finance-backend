package store

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type userStore struct {
	Client     *firestore.Client
	Collection *firestore.CollectionRef
}

func NewUserStore(client *firestore.Client) *userStore {
	return &userStore{
		Client:     client,
		Collection: client.Collection("users"),
	}
}

func (us *userStore) CreateUser(ctx context.Context, user *models.User) error {
	_, err := us.Collection.Doc(user.UID).Create(ctx, user)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return errs.NewAlreadyExistsError("user already exists")
		}
		return errs.NewDatabaseError("create", "failed to create user", err)
	}
	return nil
}

func (us *userStore) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := us.Collection.Doc(user.UID).Set(ctx, user, firestore.MergeAll)
	if err != nil {
		return errs.NewDatabaseError("update", "failed to update user", err)
	}
	return nil
}

func (us *userStore) GetUser(ctx context.Context, uid string) (*models.User, error) {
	var user models.User

	doc, err := us.Collection.Doc(uid).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errs.NewNotFoundError("user not found")
		}
		return nil, errs.NewDatabaseError("read", "failed to get user", err)
	}
	if err := doc.DataTo(&user); err != nil {
		return nil, errs.NewDatabaseError("read", "failed to parse user data", err)
	}

	return &user, nil
}
