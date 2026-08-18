package delivery

import (
	"time"

	"github.com/google/uuid"
)

// Zone is a vendor-managed city-level delivery service declaration for a store.
type Zone struct {
	ID        uuid.UUID `json:"id"`
	StoreID   uuid.UUID `json:"store_id"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Country   string    `json:"country"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertZoneRequest contains the mutable city-level service-area declaration.
type UpsertZoneRequest struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	Country  string `json:"country"`
	IsActive bool   `json:"is_active"`
}

// EligibilityRequest describes the saved delivery location city used for a store coverage lookup.
type EligibilityRequest struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

// EligibilityResponse is intentionally limited to coverage status; it contains no fee, ETA, or routing assertion.
type EligibilityResponse struct {
	Eligible bool   `json:"eligible"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Zone     *Zone  `json:"zone,omitempty"`
}
