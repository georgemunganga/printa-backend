package inventory

import "context"

// StoreRepository defines store data storage.
type StoreRepository interface {
CreateStore(ctx context.Context, s *Store) error
GetStoreByID(ctx context.Context, id string) (*Store, error)
ListStoresByVendor(ctx context.Context, vendorID string) ([]*Store, error)
}

// StoreStaffRepository defines store staff data storage.
type StoreStaffRepository interface {
AddStaff(ctx context.Context, staff *StoreStaff) error
ListStaff(ctx context.Context, storeID string) ([]*StoreStaff, error)
RemoveStaff(ctx context.Context, storeID, userID string) error
}

// ProductRepository defines vendor store product data storage.
type ProductRepository interface {
AddProduct(ctx context.Context, p *VendorStoreProduct) error
ListProducts(ctx context.Context, storeID string) ([]*VendorStoreProduct, error)
UpdateStock(ctx context.Context, id string, qty int) error
UpdateAvailability(ctx context.Context, id string, available bool) error
}
