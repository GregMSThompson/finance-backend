package handlers

import (
	"context"
)

type UserService interface {
	Register(ctx context.Context, uid, email, first, last string) error
}
