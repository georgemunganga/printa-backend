package delivery

import "context"

// ZoneRepository persists vendor store delivery service declarations.
type ZoneRepository interface {
	ListByStore(ctx context.Context, storeID string) ([]*Zone, error)
	GetByStore(ctx context.Context, id, storeID string) (*Zone, error)
	Create(ctx context.Context, zone *Zone) error
	Update(ctx context.Context, zone *Zone) error
	Delete(ctx context.Context, id, storeID string) error
	FindActiveByStoreCity(ctx context.Context, storeID, city, country string) (*Zone, error)
	HasAnyForStore(ctx context.Context, storeID string) (bool, error)
}
