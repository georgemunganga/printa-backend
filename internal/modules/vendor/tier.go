package vendor

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Tier represents a vendor subscription tier.
// @Description Tier information
// @Description with id, name, monthly_price, features, created_at, and updated_at
type Tier struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	MonthlyPrice float64         `json:"monthly_price"`
	Features     json.RawMessage `json:"features"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
