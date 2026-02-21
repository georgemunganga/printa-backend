package vendor

import (
	"time"

	"github.com/google/uuid"
)

// Vendor represents a vendor in the system.
// @Description Vendor information
// @Description with id, owner_id, tier_id, business_name, tax_id, created_at, and updated_at
type Vendor struct {
	ID           uuid.UUID `json:"id"`
	OwnerID      uuid.UUID `json:"owner_id"`
	TierID       uuid.UUID `json:"tier_id"`
	BusinessName string    `json:"business_name"`
	TaxID        string    `json:"tax_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
