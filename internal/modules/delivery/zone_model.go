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
	City      string   `json:"city"`
	Country   string   `json:"country"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// EligibilityResponse includes a server quote when exact destination coordinates are supplied.
type EligibilityResponse struct {
	Eligible bool        `json:"eligible"`
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Zone     *Zone       `json:"zone,omitempty"`
	Quote    *PriceQuote `json:"quote,omitempty"`
}
