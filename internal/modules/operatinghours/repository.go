package operatinghours

import "context"

// Repository persists store-scoped operating hours.
type Repository interface {
	List(ctx context.Context, storeID string) ([]OperatingHour, error)
	Replace(ctx context.Context, storeID string, hours []OperatingHour) error
}
