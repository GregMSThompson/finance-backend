package store

import (
	"cloud.google.com/go/firestore"
)

type userStore struct {
	Client *firestore.Client
}

func NewUserStore(client *firestore.Client) *userStore {
	return &userStore{Client: client}
}

func (*userStore) CreateUser() {

}
