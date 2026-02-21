package user

import "context"

// Service defines the interface for user-related business logic.
type Service interface {
	RegisterUser(ctx context.Context, email, password, firstName, lastName string) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
}
