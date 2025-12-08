package store

import (
	"context"

	"cloud.google.com/go/firestore"

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
	return err
}

func (us *userStore) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := us.Collection.Doc(user.UID).Set(ctx, user, firestore.MergeAll)
	return err
}

func (us *userStore) GetUser(ctx context.Context, uid string) (*models.User, error) {
	var user models.User

	doc, err := us.Collection.Doc(uid).Get(ctx)
	if err != nil {
		return nil, err
	}
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}

	return &user, nil
}
