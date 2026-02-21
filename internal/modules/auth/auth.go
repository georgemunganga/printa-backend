package auth

import "context"

// Service defines the interface for authentication-related business logic.
type Service interface {
	Login(ctx context.Context, email, password string) (string, error)
}
