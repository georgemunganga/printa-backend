package order

import "context"

// Repository defines data access for orders.
type Repository interface {
	// CreateOrder persists a new order and its items atomically in a transaction.
	CreateOrder(ctx context.Context, o *Order) error

	// GetOrderByID retrieves an order with its items by UUID.
	GetOrderByID(ctx context.Context, id string) (*Order, error)

	// GetOrderByNumber retrieves an order by its human-readable order number.
	GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error)

	// ListOrdersByStore returns all orders for a given store, optionally filtered by status.
	ListOrdersByStore(ctx context.Context, storeID string, status string) ([]*Order, error)

	// ListOrdersByCustomer returns all orders placed by a specific customer.
	ListOrdersByCustomer(ctx context.Context, customerID string) ([]*Order, error)

	// UpdateStatus advances an order to a new status.
	UpdateStatus(ctx context.Context, id string, status OrderStatus) error

	// GetProductPrice fetches the current vendor price and availability for a store product.
	GetProductPrice(ctx context.Context, vendorStoreProductID string) (price float64, available bool, err error)
}
