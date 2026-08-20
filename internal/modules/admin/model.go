package admin

import "time"

// ── Platform Stats ────────────────────────────────────────────────────────────

// PlatformStats is a snapshot of key platform metrics for the admin dashboard.
type PlatformStats struct {
	TotalUsers          int     `json:"total_users"`
	TotalVendors        int     `json:"total_vendors"`
	TotalStores         int     `json:"total_stores"`
	TotalOrders         int     `json:"total_orders"`
	TotalRevenue        float64 `json:"total_revenue"`
	ActiveSubscriptions int     `json:"active_subscriptions"`
	PendingOrders       int     `json:"pending_orders"`
	ProductionJobs      int     `json:"active_production_jobs"`
}

// ── User Management ───────────────────────────────────────────────────────────

type AdminUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	Phone     string    `json:"phone,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateUserRequest struct {
	Role     string `json:"role,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
}

// CreateAdministratorRequest intentionally contains no password. Administrator
// access is OTP-only and the required legacy password hash is internally random.
type CreateAdministratorRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type UpdateAdministratorStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// ── Vendor Management ─────────────────────────────────────────────────────────

type AdminVendor struct {
	ID           string    `json:"id"`
	BusinessName string    `json:"business_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	Status       string    `json:"status"`
	TierName     string    `json:"tier_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type UpdateVendorStatusRequest struct {
	Status string `json:"status"` // ACTIVE | SUSPENDED | BANNED
}

// ── Order Management ─────────────────────────────────────────────────────────

type AdminOrder struct {
	ID          string    `json:"id"`
	OrderNumber string    `json:"order_number"`
	StoreID     string    `json:"store_id"`
	StoreName   string    `json:"store_name,omitempty"`
	CustomerID  string    `json:"customer_id,omitempty"`
	Status      string    `json:"status"`
	TotalAmount float64   `json:"total_amount"`
	CreatedAt   time.Time `json:"created_at"`
}

// ── Subscription Management ───────────────────────────────────────────────────

type AdminSubscription struct {
	ID         string    `json:"id"`
	VendorID   string    `json:"vendor_id"`
	VendorName string    `json:"vendor_name,omitempty"`
	TierName   string    `json:"tier_name"`
	Status     string    `json:"status"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	AutoRenew  bool      `json:"auto_renew"`
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

type AuditLog struct {
	ID         string    `json:"id"`
	AdminID    string    `json:"admin_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"created_at"`
}
