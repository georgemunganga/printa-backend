package user

import "context"

// Repository defines the interface for user data storage.
type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
	DeactivateUser(ctx context.Context, id string) error
}
