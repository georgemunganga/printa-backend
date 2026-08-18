package delivery

import (
	"time"

	"github.com/google/uuid"
)

// Location is a durable delivery address owned by a customer.
type Location struct {
	ID             uuid.UUID `json:"id"`
	CustomerID     uuid.UUID `json:"customer_id"`
	Label          string    `json:"label"`
	RecipientName  string    `json:"recipient_name"`
	RecipientPhone string    `json:"recipient_phone"`
	AddressLine1   string    `json:"address_line1"`
	AddressLine2   string    `json:"address_line2,omitempty"`
	City           string    `json:"city"`
	Country        string    `json:"country"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UpsertLocationRequest contains mutable customer delivery-location attributes.
type UpsertLocationRequest struct {
	Label          string   `json:"label"`
	RecipientName  string   `json:"recipient_name"`
	RecipientPhone string   `json:"recipient_phone"`
	AddressLine1   string   `json:"address_line1"`
	AddressLine2   string   `json:"address_line2"`
	City           string   `json:"city"`
	Country        string   `json:"country"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	IsDefault      bool     `json:"is_default"`
}
