package user

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
// @Description User information
// @Description with id, email, first_name, last_name, created_at, and updated_at
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"first_name,omitempty"`
	LastName     string    `json:"last_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
