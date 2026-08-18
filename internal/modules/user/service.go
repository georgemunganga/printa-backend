package user

import "context"

// Service defines the interface for user-related business logic.
type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type Service interface {
	RegisterUser(ctx context.Context, email, password, firstName, lastName, role string) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
}
