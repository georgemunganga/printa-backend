package user

import (
	"context"
	"errors"
	"testing"
)

type memoryRepository struct {
	created *User
}

func (r *memoryRepository) CreateUser(_ context.Context, u *User) error {
	r.created = u
	return nil
}
func (r *memoryRepository) CreateOAuthUser(_ context.Context, u *User) error {
	return r.CreateUser(context.Background(), u)
}
func (r *memoryRepository) GetUserByEmail(context.Context, string) (*User, error) {
	return nil, errors.New("not found")
}
func (r *memoryRepository) GetUserByPhone(context.Context, string) (*User, error) {
	return nil, errors.New("not found")
}
func (r *memoryRepository) GetUserByID(context.Context, string) (*User, error) {
	return nil, errors.New("not found")
}
func (r *memoryRepository) ListUsers(context.Context) ([]*User, error)            { return nil, nil }
func (r *memoryRepository) UpdateUserRole(context.Context, string, string) error  { return nil }
func (r *memoryRepository) PromoteUserRole(context.Context, string, string) error { return nil }
func (r *memoryRepository) DeactivateUser(context.Context, string) error          { return nil }

func TestRegisterUserAllowsOnlySelfServiceRoles(t *testing.T) {
	for _, role := range []string{"", "CUSTOMER", "VENDOR"} {
		t.Run(role, func(t *testing.T) {
			repo := &memoryRepository{}
			svc := NewService(repo)

			u, err := svc.RegisterUser(context.Background(), "person@example.com", "password", "First", "Last", role)
			if err != nil {
				t.Fatalf("RegisterUser() error = %v", err)
			}
			if repo.created == nil || u != repo.created {
				t.Fatal("RegisterUser() did not persist the new user")
			}
			if role == "" && u.Role != "CUSTOMER" {
				t.Fatalf("default role = %q, want CUSTOMER", u.Role)
			}
		})
	}
}

func TestRegisterUserRejectsPrivilegedSelfAssignment(t *testing.T) {
	for _, role := range []string{"ADMIN", "STAFF", "CASHIER", "UNKNOWN"} {
		t.Run(role, func(t *testing.T) {
			repo := &memoryRepository{}
			svc := NewService(repo)

			if _, err := svc.RegisterUser(context.Background(), "person@example.com", "password", "First", "Last", role); err == nil {
				t.Fatalf("RegisterUser() accepted privileged role %q", role)
			}
			if repo.created != nil {
				t.Fatalf("RegisterUser() persisted privileged role %q", role)
			}
		})
	}
}
