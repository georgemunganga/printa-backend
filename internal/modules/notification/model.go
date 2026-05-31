package notification

import "time"

// Type classifies the notification for routing and display purposes.
type Type string

const (
	TypeOrderPlaced        Type = "ORDER_PLACED"
	TypeOrderStatusChanged Type = "ORDER_STATUS_CHANGED"
	TypeOrderCancelled     Type = "ORDER_CANCELLED"
	TypePaymentReceived    Type = "PAYMENT_RECEIVED"
	TypePaymentFailed      Type = "PAYMENT_FAILED"
	TypePaymentRefunded    Type = "PAYMENT_REFUNDED"
	TypeProductionJobReady Type = "PRODUCTION_JOB_READY"
	TypeProductionComplete Type = "PRODUCTION_COMPLETE"
	TypeSubscriptionExpiry Type = "SUBSCRIPTION_EXPIRY"
	TypeSubscriptionRenewed Type = "SUBSCRIPTION_RENEWED"
	TypeInvoiceGenerated   Type = "INVOICE_GENERATED"
	TypeInvoiceDue         Type = "INVOICE_DUE"
	TypeVendorSuspended    Type = "VENDOR_SUSPENDED"
	TypeSystemAlert        Type = "SYSTEM_ALERT"
	TypeCustom             Type = "CUSTOM"
)

// Status tracks the lifecycle of a notification.
type Status string

const (
	StatusUnread    Status = "UNREAD"
	StatusRead      Status = "READ"
	StatusDismissed Status = "DISMISSED"
)

// Priority controls urgency and display ordering.
type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityNormal Priority = "NORMAL"
	PriorityHigh   Priority = "HIGH"
	PriorityUrgent Priority = "URGENT"
)

// Notification is the core domain entity stored in the database.
type Notification struct {
	ID         string            `json:"id"`
	RecipientID string           `json:"recipient_id"`          // user UUID
	Type       Type              `json:"type"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Status     Status            `json:"status"`
	Priority   Priority          `json:"priority"`
	Metadata   map[string]string `json:"metadata,omitempty"`    // e.g. {"order_id": "...", "store_id": "..."}
	ReadAt     *time.Time        `json:"read_at,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// CreateRequest is the input for creating a new notification.
type CreateRequest struct {
	RecipientID string            `json:"recipient_id"`
	Type        Type              `json:"type"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Priority    Priority          `json:"priority,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// BulkCreateRequest creates the same notification for multiple recipients.
type BulkCreateRequest struct {
	RecipientIDs []string          `json:"recipient_ids"`
	Type         Type              `json:"type"`
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	Priority     Priority          `json:"priority,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ListFilter is used to query notifications for a recipient.
type ListFilter struct {
	RecipientID string
	Status      Status
	Type        Type
	Limit       int
	Offset      int
}

// UnreadCount is returned for the notification badge count.
type UnreadCount struct {
	RecipientID string `json:"recipient_id"`
	Count       int    `json:"unread_count"`
}

// Event is a domain event payload used to trigger notifications internally.
// Other modules call notification.NewEvent(...) to fire events without
// directly importing the notification service.
type Event struct {
	Type        Type
	RecipientID string
	Title       string
	Body        string
	Priority    Priority
	Metadata    map[string]string
}
