package vendor

import (
	"time"

	"github.com/google/uuid"
)

// Vendor represents a vendor in the Printa platform.
// @Description Vendor information with ownership, subscription tier, and business details.
type Vendor struct {
	ID           uuid.UUID   `json:"id"`
	OwnerID      uuid.UUID   `json:"owner_id"`
	TierID       uuid.UUID   `json:"tier_id"`
	BusinessName string      `json:"business_name"`
	TaxID        string      `json:"tax_id,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	FirstStore   *FirstStore `json:"first_store,omitempty"`
}

// FirstStore is the first physical storefront persisted with a vendor during
// onboarding. It intentionally mirrors only the store attributes collected by
// the onboarding workflow and does not expose inventory-module internals.
type FirstStore struct {
	ID        uuid.UUID `json:"id"`
	VendorID  uuid.UUID `json:"vendor_id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	Country   string    `json:"country"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FirstStoreInput is the validated storefront input collected from a vendor
// during onboarding. A complete value is required when atomic onboarding is
// requested.
type FirstStoreInput struct {
	Name         string
	Address      string
	City         string
	Country      string
	Latitude     *float64
	Longitude    *float64
	OwnerPIN     string
	OwnerPINHash string
}
