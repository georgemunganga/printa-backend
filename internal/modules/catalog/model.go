package catalog

import (
"encoding/json"
"time"

"github.com/google/uuid"
)

// PlatformProduct is a product in the master catalog managed by the Printa platform.
type PlatformProduct struct {
ID          uuid.UUID       `json:"id"`
Name        string          `json:"name"`
Description string          `json:"description,omitempty"`
Category    string          `json:"category"`
BasePrice   float64         `json:"base_price"`
Currency    string          `json:"currency"`
SKU         string          `json:"sku,omitempty"`
ImageURL    string          `json:"image_url,omitempty"`
IsActive    bool            `json:"is_active"`
Attributes  json.RawMessage `json:"attributes,omitempty"`
CreatedAt   time.Time       `json:"created_at"`
UpdatedAt   time.Time       `json:"updated_at"`
}
