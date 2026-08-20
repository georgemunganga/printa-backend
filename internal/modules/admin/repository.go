package admin

import "context"

// Repository defines all data access operations for the admin module.
type Repository interface {
	// Platform stats
	GetPlatformStats(ctx context.Context) (*PlatformStats, error)

	// User management
	ListUsers(ctx context.Context, role, search string, limit, offset int) ([]AdminUser, int, error)
	GetUser(ctx context.Context, id string) (*AdminUser, error)
	UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*AdminUser, error)
	DeactivateUser(ctx context.Context, id string) error
	CreateAdministrator(ctx context.Context, request CreateAdministratorRequest, passwordHash string) (*AdminUser, error)
	CountActiveAdministrators(ctx context.Context) (int, error)

	// Vendor management
	ListVendors(ctx context.Context, status, search string, limit, offset int) ([]AdminVendor, int, error)
	GetVendor(ctx context.Context, id string) (*AdminVendor, error)
	UpdateVendorStatus(ctx context.Context, id, status string) (*AdminVendor, error)

	// Order management
	ListOrders(ctx context.Context, status string, limit, offset int) ([]AdminOrder, int, error)
	GetOrder(ctx context.Context, id string) (*AdminOrder, error)

	// Subscription management
	ListSubscriptions(ctx context.Context, status string, limit, offset int) ([]AdminSubscription, int, error)

	// Audit log
	CreateAuditLog(ctx context.Context, log AuditLog) error
	ListAuditLogs(ctx context.Context, adminID, targetType string, limit, offset int) ([]AuditLog, int, error)
}
