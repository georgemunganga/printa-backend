package paymentmethod

import (
	"time"

	"github.com/google/uuid"
)

type Method struct {
	ID          uuid.UUID `json:"id"`
	CustomerID  uuid.UUID `json:"customer_id"`
	Provider    string    `json:"provider"`
	PhoneNumber string    `json:"phone_number"`
	Label       string    `json:"label"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpsertRequest struct {
	Provider    string `json:"provider"`
	PhoneNumber string `json:"phone_number"`
	Label       string `json:"label"`
	IsDefault   bool   `json:"is_default"`
}
