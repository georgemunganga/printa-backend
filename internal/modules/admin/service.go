package admin

import (
	"context"
	"fmt"
)

// Service defines all admin business logic operations.
type Service interface {
	GetPlatformStats(ctx context.Context) (*PlatformStats, error)

	// Users
	ListUsers(ctx context.Context, role, search string, page, pageSize int) ([]AdminUser, int, error)
	GetUser(ctx context.Context, id string) (*AdminUser, error)
	UpdateUser(ctx context.Context, adminID, targetID string, req UpdateUserRequest) (*AdminUser, error)
	DeactivateUser(ctx context.Context, adminID, targetID string) error

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

func (s *service) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	return s.repo.GetUser(ctx, id)
}

func (s *service) UpdateUser(ctx context.Context, adminID, targetID string, req UpdateUserRequest) (*AdminUser, error) {
	validRoles := map[string]bool{"ADMIN": true, "VENDOR": true, "STAFF": true, "CASHIER": true, "CUSTOMER": true}
	if req.Role != "" && !validRoles[req.Role] {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}
	user, err := s.repo.UpdateUser(ctx, targetID, req)
	if err != nil {
		return nil, err
	}
	_ = s.repo.CreateAuditLog(ctx, AuditLog{
		AdminID:    adminID,
		Action:     "UPDATE_USER",
		TargetType: "user",
		TargetID:   targetID,
		Details:    fmt.Sprintf("role=%s", req.Role),
	})
	return user, nil
}

func (s *service) DeactivateUser(ctx context.Context, adminID, targetID string) error {
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
