package catalog

import "context"

// Repository defines the interface for platform product data storage.
type Repository interface {
Create(ctx context.Context, p *PlatformProduct) error
GetByID(ctx context.Context, id string) (*PlatformProduct, error)
List(ctx context.Context, category string, activeOnly bool) ([]*PlatformProduct, error)
Update(ctx context.Context, p *PlatformProduct) error
}
