package delivery

import "context"

// Repository persists customer-owned delivery locations.
type Repository interface {
	ListByCustomer(ctx context.Context, customerID string) ([]*Location, error)
	GetByCustomer(ctx context.Context, id, customerID string) (*Location, error)
	Create(ctx context.Context, location *Location) error
	Update(ctx context.Context, location *Location) error
	Delete(ctx context.Context, id, customerID string) error
	SetDefault(ctx context.Context, id, customerID string) error
	PromoteEarliestAsDefault(ctx context.Context, customerID string) error
}
