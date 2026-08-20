package admin

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Service defines all admin business logic operations.
type Service interface {
	GetPlatformStats(ctx context.Context) (*PlatformStats, error)

	// Users
	ListUsers(ctx context.Context, role, search string, page, pageSize int) ([]AdminUser, int, error)
	ListAdministrators(ctx context.Context, search string, page, pageSize int) ([]AdminUser, int, error)
	GetUser(ctx context.Context, id string) (*AdminUser, error)
	UpdateUser(ctx context.Context, adminID, targetID string, req UpdateUserRequest) (*AdminUser, error)
	DeactivateUser(ctx context.Context, adminID, targetID string) error
	CreateAdministrator(ctx context.Context, adminID string, request CreateAdministratorRequest) (*AdminUser, error)
	UpdateAdministratorStatus(ctx context.Context, adminID, targetID string, request UpdateAdministratorStatusRequest) (*AdminUser, error)

	// Vendors
	ListVendors(ctx context.Context, status, search string, page, pageSize int) ([]AdminVendor, int, error)
	GetVendor(ctx context.Context, id string) (*AdminVendor, error)
	UpdateVendorStatus(ctx context.Context, adminID, vendorID, status string) (*AdminVendor, error)

	// Orders
	ListOrders(ctx context.Context, status string, page, pageSize int) ([]AdminOrder, int, error)
	GetOrder(ctx context.Context, id string) (*AdminOrder, error)

	// Subscriptions
	ListSubscriptions(ctx context.Context, status string, page, pageSize int) ([]AdminSubscription, int, error)

	// Audit
	ListAuditLogs(ctx context.Context, adminID, targetType string, page, pageSize int) ([]AuditLog, int, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	return s.repo.GetPlatformStats(ctx)
}

// ── Users ─────────────────────────────────────────────────────────────────────

func (s *service) ListUsers(ctx context.Context, role, search string, page, pageSize int) ([]AdminUser, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListUsers(ctx, role, search, pageSize, offset)
}

func (s *service) ListAdministrators(ctx context.Context, search string, page, pageSize int) ([]AdminUser, int, error) {
	return s.ListUsers(ctx, "ADMIN", search, page, pageSize)
}

func (s *service) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	return s.repo.GetUser(ctx, id)
}

func (s *service) UpdateUser(ctx context.Context, adminID, targetID string, req UpdateUserRequest) (*AdminUser, error) {
	if req.Role != "" {
		return nil, fmt.Errorf("role changes are not allowed in the general user workflow")
	}
	if req.IsActive == nil {
		return nil, fmt.Errorf("an account status is required")
	}
	target, err := s.repo.GetUser(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == "ADMIN" {
		return nil, fmt.Errorf("administrator identities must be managed through the administrator workflow")
	}
	user, err := s.repo.UpdateUser(ctx, targetID, req)
	if err != nil {
		return nil, err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "UPDATE_USER_STATUS",
		TargetType: "user",
		TargetID:   targetID,
		Details:    fmt.Sprintf("is_active=%t", *req.IsActive),
	})
	return user, nil
}

func (s *service) DeactivateUser(ctx context.Context, adminID, targetID string) error {
	target, err := s.repo.GetUser(ctx, targetID)
	if err != nil {
		return err
	}
	if target.Role == "ADMIN" {
		return fmt.Errorf("administrator identities must be managed through the administrator workflow")
	}
	if err := s.repo.DeactivateUser(ctx, targetID); err != nil {
		return err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "DEACTIVATE_USER",
		TargetType: "user",
		TargetID:   targetID,
		Details:    "user deactivated by admin",
	})
	return nil
}

func (s *service) CreateAdministrator(ctx context.Context, adminID string, request CreateAdministratorRequest) (*AdminUser, error) {
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.FirstName = strings.TrimSpace(request.FirstName)
	request.LastName = strings.TrimSpace(request.LastName)
	parsed, err := mail.ParseAddress(request.Email)
	if err != nil || parsed.Address != request.Email {
		return nil, fmt.Errorf("a valid administrator email is required")
	}
	if request.FirstName == "" || request.LastName == "" {
		return nil, fmt.Errorf("administrator first and last names are required")
	}

	legacySecret := uuid.NewString() + uuid.NewString()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(legacySecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("prepare administrator account: %w", err)
	}
	administrator, err := s.repo.CreateAdministrator(ctx, request, string(passwordHash))
	if err != nil {
		return nil, err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "CREATE_ADMINISTRATOR",
		TargetType: "administrator",
		TargetID:   administrator.ID,
		Details:    fmt.Sprintf("email=%s", administrator.Email),
	})
	return administrator, nil
}

func (s *service) UpdateAdministratorStatus(ctx context.Context, adminID, targetID string, request UpdateAdministratorStatusRequest) (*AdminUser, error) {
	target, err := s.repo.GetUser(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role != "ADMIN" {
		return nil, fmt.Errorf("the target is not an administrator")
	}
	if targetID == adminID && !request.IsActive {
		return nil, fmt.Errorf("you cannot deactivate your own administrator account")
	}
	if !request.IsActive && target.IsActive {
		activeAdministrators, err := s.repo.CountActiveAdministrators(ctx)
		if err != nil {
			return nil, err
		}
		if activeAdministrators <= 1 {
			return nil, fmt.Errorf("at least one active administrator must remain")
		}
	}

	isActive := request.IsActive
	administrator, err := s.repo.UpdateUser(ctx, targetID, UpdateUserRequest{IsActive: &isActive})
	if err != nil {
		return nil, err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "UPDATE_ADMINISTRATOR_STATUS",
		TargetType: "administrator",
		TargetID:   targetID,
		Details:    fmt.Sprintf("is_active=%t", request.IsActive),
	})
	return administrator, nil
}

// ── Vendors ───────────────────────────────────────────────────────────────────

func (s *service) ListVendors(ctx context.Context, status, search string, page, pageSize int) ([]AdminVendor, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListVendors(ctx, status, search, pageSize, offset)
}

func (s *service) GetVendor(ctx context.Context, id string) (*AdminVendor, error) {
	return s.repo.GetVendor(ctx, id)
}

func (s *service) UpdateVendorStatus(ctx context.Context, adminID, vendorID, status string) (*AdminVendor, error) {
	validStatuses := map[string]bool{"ACTIVE": true, "SUSPENDED": true, "BANNED": true}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid vendor status: %s — must be ACTIVE, SUSPENDED, or BANNED", status)
	}
	vendor, err := s.repo.UpdateVendorStatus(ctx, vendorID, status)
	if err != nil {
		return nil, err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "UPDATE_VENDOR_STATUS",
		TargetType: "vendor",
		TargetID:   vendorID,
		Details:    fmt.Sprintf("status=%s", status),
	})
	return vendor, nil
}

// ── Orders ────────────────────────────────────────────────────────────────────

func (s *service) ListOrders(ctx context.Context, status string, page, pageSize int) ([]AdminOrder, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListOrders(ctx, status, pageSize, offset)
}

func (s *service) GetOrder(ctx context.Context, id string) (*AdminOrder, error) {
	return s.repo.GetOrder(ctx, id)
}

// ── Subscriptions ─────────────────────────────────────────────────────────────

func (s *service) ListSubscriptions(ctx context.Context, status string, page, pageSize int) ([]AdminSubscription, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListSubscriptions(ctx, status, pageSize, offset)
}

// ── Audit ─────────────────────────────────────────────────────────────────────

func (s *service) ListAuditLogs(ctx context.Context, adminID, targetType string, page, pageSize int) ([]AuditLog, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListAuditLogs(ctx, adminID, targetType, pageSize, offset)
}
