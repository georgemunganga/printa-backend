package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service defines inventory business logic for stores, staff, and products.
type Service interface {
	// Store operations
	CreateStore(ctx context.Context, req CreateStoreRequest) (*Store, error)
	GetStore(ctx context.Context, id string) (*Store, error)
	ListStores(ctx context.Context, vendorID string) ([]*Store, error)
	ListStorefrontStores(ctx context.Context) ([]*Store, error)
	ListStorefrontProducts(ctx context.Context, storeID string) ([]*StorefrontProduct, error)
	UpdateStore(ctx context.Context, id string, req UpdateStoreRequest) (*Store, error)
	DeactivateStore(ctx context.Context, id string) error

	// Staff operations
	AddStaff(ctx context.Context, storeID, userID, role string) (*StoreStaff, error)
	ListStaff(ctx context.Context, storeID string) ([]*StoreStaff, error)
	RemoveStaff(ctx context.Context, storeID, userID string) error
	UpdateStaffRole(ctx context.Context, storeID, userID, role string) error

	// Product listing operations
	AddProduct(ctx context.Context, req AddProductRequest) (*VendorStoreProduct, error)
	GetProduct(ctx context.Context, id string) (*VendorStoreProduct, error)
	ListProducts(ctx context.Context, storeID string) ([]*VendorStoreProduct, error)
	UpdateStock(ctx context.Context, productID string, qty int) error
	UpdateVendorPrice(ctx context.Context, productID string, price float64) error
	SetAvailability(ctx context.Context, productID string, available bool) error
}

// CreateStoreRequest holds data for creating a store.
type CreateStoreRequest struct {
	VendorID    string   `json:"vendor_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Phone       string   `json:"phone"`
	Email       string   `json:"email"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
}

// UpdateStoreRequest holds the mutable attributes of a vendor store.
// Ownership and active-state controls are intentionally excluded from this public request.
type UpdateStoreRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Phone       string   `json:"phone"`
	Email       string   `json:"email"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
}

// AddProductRequest holds data for listing a product in a store.
type AddProductRequest struct {
	StoreID           string  `json:"store_id"`
	PlatformProductID string  `json:"platform_product_id"`
	VendorPrice       float64 `json:"vendor_price"`
	Currency          string  `json:"currency"`
	StockQuantity     int     `json:"stock_quantity"`
}

type service struct {
	storeRepo   StoreRepository
	staffRepo   StoreStaffRepository
	productRepo ProductRepository
}

// NewService creates a new inventory service.
func NewService(storeRepo StoreRepository, staffRepo StoreStaffRepository, productRepo ProductRepository) Service {
	return &service{
		storeRepo:   storeRepo,
		staffRepo:   staffRepo,
		productRepo: productRepo,
	}
}

func (s *service) CreateStore(ctx context.Context, req CreateStoreRequest) (*Store, error) {
	if err := validateCoordinates(req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	vendorID, err := uuid.Parse(req.VendorID)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor_id: %w", err)
	}
	country := req.Country
	if country == "" {
		country = "Zambia"
	}
	store := &Store{
		ID:          uuid.New(),
		VendorID:    vendorID,
		Name:        req.Name,
		Description: req.Description,
		Address:     req.Address,
		City:        req.City,
		Country:     country,
		Phone:       req.Phone,
		Email:       req.Email,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		IsActive:    true,
	}
	if err := s.storeRepo.CreateStore(ctx, store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *service) GetStore(ctx context.Context, id string) (*Store, error) {
	return s.storeRepo.GetStoreByID(ctx, id)
}

func (s *service) ListStores(ctx context.Context, vendorID string) ([]*Store, error) {
	return s.storeRepo.ListStoresByVendor(ctx, vendorID)
}

func (s *service) ListStorefrontStores(ctx context.Context) ([]*Store, error) {
	return s.storeRepo.ListActiveStores(ctx)
}

func (s *service) ListStorefrontProducts(ctx context.Context, storeID string) ([]*StorefrontProduct, error) {
	return s.productRepo.ListAvailableStorefrontProducts(ctx, storeID)
}

func (s *service) UpdateStore(ctx context.Context, id string, req UpdateStoreRequest) (*Store, error) {
	if err := validateCoordinates(req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	store, err := s.storeRepo.GetStoreByID(ctx, id)
	if err != nil {
		return nil, err
	}

	store.Name = req.Name
	store.Description = req.Description
	store.Address = req.Address
	store.City = req.City
	store.Country = req.Country
	store.Phone = req.Phone
	store.Email = req.Email
	store.Latitude = req.Latitude
	store.Longitude = req.Longitude
	if store.Country == "" {
		store.Country = "Zambia"
	}

	if err := s.storeRepo.UpdateStore(ctx, store); err != nil {
		return nil, err
	}
	return s.storeRepo.GetStoreByID(ctx, id)
}

func validateCoordinates(latitude, longitude *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return fmt.Errorf("latitude and longitude must be supplied together")
	}
	if latitude != nil && (*latitude < -90 || *latitude > 90) {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if longitude != nil && (*longitude < -180 || *longitude > 180) {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

func (s *service) DeactivateStore(ctx context.Context, id string) error {
	return s.storeRepo.DeactivateStore(ctx, id)
}

func (s *service) AddStaff(ctx context.Context, storeID, userID, role string) (*StoreStaff, error) {
	sid, err := uuid.Parse(storeID)
	if err != nil {
		return nil, fmt.Errorf("invalid store_id: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}
	if role == "" {
		role = "STAFF"
	}
	staff := &StoreStaff{
		ID:      uuid.New(),
		StoreID: sid,
		UserID:  uid,
		Role:    role,
	}
	if err := s.staffRepo.AddStaff(ctx, staff); err != nil {
		return nil, err
	}
	return staff, nil
}

func (s *service) ListStaff(ctx context.Context, storeID string) ([]*StoreStaff, error) {
	return s.staffRepo.ListStaff(ctx, storeID)
}

func (s *service) RemoveStaff(ctx context.Context, storeID, userID string) error {
	return s.staffRepo.RemoveStaff(ctx, storeID, userID)
}

func (s *service) UpdateStaffRole(ctx context.Context, storeID, userID, role string) error {
	if _, ok := validStoreStaffRole(role); !ok {
		return fmt.Errorf("role must be STAFF, MANAGER, or CASHIER")
	}
	return s.staffRepo.UpdateStaffRole(ctx, storeID, userID, role)
}

func (s *service) AddProduct(ctx context.Context, req AddProductRequest) (*VendorStoreProduct, error) {
	sid, err := uuid.Parse(req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid store_id: %w", err)
	}
	pid, err := uuid.Parse(req.PlatformProductID)
	if err != nil {
		return nil, fmt.Errorf("invalid platform_product_id: %w", err)
	}
	currency := req.Currency
	if currency == "" {
		currency = "ZMW"
	}
	p := &VendorStoreProduct{
		ID:                uuid.New(),
		StoreID:           sid,
		PlatformProductID: pid,
		VendorPrice:       req.VendorPrice,
		Currency:          currency,
		StockQuantity:     req.StockQuantity,
		IsAvailable:       true,
	}
	if err := s.productRepo.AddProduct(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *service) GetProduct(ctx context.Context, id string) (*VendorStoreProduct, error) {
	return s.productRepo.GetProductByID(ctx, id)
}

func (s *service) ListProducts(ctx context.Context, storeID string) ([]*VendorStoreProduct, error) {
	return s.productRepo.ListProducts(ctx, storeID)
}

func (s *service) UpdateStock(ctx context.Context, productID string, qty int) error {
	if qty < 0 {
		return fmt.Errorf("quantity cannot be negative")
	}
	return s.productRepo.UpdateStock(ctx, productID, qty)
}

func (s *service) UpdateVendorPrice(ctx context.Context, productID string, price float64) error {
	if price <= 0 {
		return fmt.Errorf("vendor price must be greater than zero")
	}
	return s.productRepo.UpdateVendorPrice(ctx, productID, price)
}

func (s *service) SetAvailability(ctx context.Context, productID string, available bool) error {
	return s.productRepo.UpdateAvailability(ctx, productID, available)
}
