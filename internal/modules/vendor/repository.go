package vendor

import "context"

// Repository defines the interface for vendor data storage.
type Repository interface {
	CreateVendor(ctx context.Context, vendor *Vendor) error
	GetVendorByOwnerID(ctx context.Context, ownerID string) (*Vendor, error)
}
