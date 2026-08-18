package inventory

import (
	"time"

	"github.com/google/uuid"
)

// Store represents a physical or virtual print store owned by a vendor.
type Store struct {
	ID          uuid.UUID `json:"id"`
	VendorID    uuid.UUID `json:"vendor_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Address     string    `json:"address,omitempty"`
	City        string    `json:"city,omitempty"`
	Country     string    `json:"country"`
	Phone       string    `json:"phone,omitempty"`
	Email       string    `json:"email,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StoreStaff links a user to a store with a role.
type StoreStaff struct {
	ID        uuid.UUID `json:"id"`
	StoreID   uuid.UUID `json:"store_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"` // MANAGER, STAFF, CASHIER
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	Email     string    `json:"email,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VendorStoreProduct is a platform product listed in a vendor's store with custom pricing.
type VendorStoreProduct struct {
	ID                uuid.UUID `json:"id"`
	StoreID           uuid.UUID `json:"store_id"`
	PlatformProductID uuid.UUID `json:"platform_product_id"`
	VendorPrice       float64   `json:"vendor_price"`
	Currency          string    `json:"currency"`
	StockQuantity     int       `json:"stock_quantity"`
	IsAvailable       bool      `json:"is_available"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// StorefrontProduct is the customer-safe, joined store-product representation.
type StorefrontProduct struct {
	ID          uuid.UUID `json:"id"`
	StoreID     uuid.UUID `json:"store_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	ImageURL    string    `json:"image_url,omitempty"`
	InStock     bool      `json:"in_stock"`
}
