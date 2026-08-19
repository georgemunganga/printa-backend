package vendor

import "context"

// Repository defines the vendor persistence operations.
type Repository interface {
	CreateVendor(ctx context.Context, vendor *Vendor) error
	GetVendorByOwnerID(ctx context.Context, ownerID string) (*Vendor, error)
	EnsureVendorWithFirstStore(ctx context.Context, candidate *Vendor, firstStore FirstStoreInput) (*Vendor, error)
}
