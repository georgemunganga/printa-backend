package order

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusPending      OrderStatus = "PENDING"
	StatusConfirmed    OrderStatus = "CONFIRMED"
	StatusInProduction OrderStatus = "IN_PRODUCTION"
	StatusReady        OrderStatus = "READY"
	StatusDelivered    OrderStatus = "DELIVERED"
	StatusCancelled    OrderStatus = "CANCELLED"
)

// OrderChannel indicates how the order was placed.
type OrderChannel string

const (
	ChannelOnline OrderChannel = "ONLINE"
	ChannelPOS    OrderChannel = "POS"
	ChannelKiosk  OrderChannel = "KIOSK"
)

// Order represents a customer's print order at a store.
type Order struct {
	ID              uuid.UUID       `json:"id"`
	StoreID         uuid.UUID       `json:"store_id"`
	CustomerID      *uuid.UUID      `json:"customer_id,omitempty"` // nil for walk-in POS orders
	OrderNumber     string          `json:"order_number"`
	Status          OrderStatus     `json:"status"`
	Channel         OrderChannel    `json:"channel"`
	Subtotal        float64         `json:"subtotal"`
	Discount        float64         `json:"discount"`
	Tax             float64         `json:"tax"`
	Total           float64         `json:"total"`
	Currency        string          `json:"currency"`
	Notes           string          `json:"notes,omitempty"`
	DeliveryAddress json.RawMessage `json:"delivery_address,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	Items           []*OrderItem    `json:"items,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// OrderItem is a single line item within an order.
type OrderItem struct {
	ID                   uuid.UUID       `json:"id"`
	OrderID              uuid.UUID       `json:"order_id"`
	VendorStoreProductID uuid.UUID       `json:"vendor_store_product_id"`
	Quantity             int             `json:"quantity"`
	UnitPrice            float64         `json:"unit_price"`
	LineTotal            float64         `json:"line_total"`
	Customisation        json.RawMessage `json:"customisation,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// CartItem is a transient struct used during checkout to describe what a customer wants.
type CartItem struct {
	VendorStoreProductID string          `json:"vendor_store_product_id"`
	Quantity             int             `json:"quantity"`
	Customisation        json.RawMessage `json:"customisation,omitempty"`
}

// PlaceOrderRequest is the payload for creating a new order.
type PlaceOrderRequest struct {
	StoreID         string          `json:"store_id"`
	CustomerID      string          `json:"customer_id,omitempty"` // optional for POS
	Channel         string          `json:"channel"`
	Items           []CartItem      `json:"items"`
	Notes           string          `json:"notes,omitempty"`
	DeliveryAddress json.RawMessage `json:"delivery_address,omitempty"`
	Discount        float64         `json:"discount,omitempty"`
}

// UpdateStatusRequest is the payload for advancing an order's status.
type UpdateStatusRequest struct {
	Status string `json:"status"`
}
