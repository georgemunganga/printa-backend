package vendor

import "context"

// TierRepository defines the interface for vendor tier data storage.
type TierRepository interface {
	GetTiers(ctx context.Context) ([]*Tier, error)
	GetTierByName(ctx context.Context, name string) (*Tier, error)
}
