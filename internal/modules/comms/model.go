package comms

import "time"

// ChannelType identifies the delivery channel.
type ChannelType string

const (
	ChannelEmail    ChannelType = "EMAIL"
	ChannelSMS      ChannelType = "SMS"
	ChannelPush     ChannelType = "PUSH"
	ChannelWhatsApp ChannelType = "WHATSAPP"
	ChannelInApp    ChannelType = "IN_APP" // triggers notification module
)

// DeliveryStatus tracks the lifecycle of a single delivery attempt.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "PENDING"
	DeliveryQueued    DeliveryStatus = "QUEUED"
	DeliverySent      DeliveryStatus = "SENT"
	DeliveryDelivered DeliveryStatus = "DELIVERED"
	DeliveryFailed    DeliveryStatus = "FAILED"
	DeliveryBounced   DeliveryStatus = "BOUNCED"
)

// Message is the provider-agnostic payload sent through any channel.
type Message struct {
	// Routing
	Channel     ChannelType `json:"channel"`
	Recipient   string      `json:"recipient"`    // email address, phone number, device token, or user_id
	RecipientID string      `json:"recipient_id"` // internal user UUID (optional, for audit)

	// Content
	Subject  string            `json:"subject,omitempty"`  // email subject / push title
	Body     string            `json:"body"`               // plain text body
	HTMLBody string            `json:"html_body,omitempty"` // email HTML body
	Metadata map[string]string `json:"metadata,omitempty"` // provider-specific extras

	// Tracing
	IdempotencyKey string `json:"idempotency_key,omitempty"` // prevent duplicate sends
	TemplateID     string `json:"template_id,omitempty"`     // provider template reference
}

// DeliveryLog records every send attempt for audit and retry purposes.
type DeliveryLog struct {
	ID             string         `json:"id"`
	Channel        ChannelType    `json:"channel"`
	Recipient      string         `json:"recipient"`
	RecipientID    string         `json:"recipient_id,omitempty"`
	Subject        string         `json:"subject,omitempty"`
	Body           string         `json:"body"`
	Status         DeliveryStatus `json:"status"`
	ProviderRef    string         `json:"provider_ref,omitempty"`  // provider message ID
	ErrorMessage   string         `json:"error_message,omitempty"` // last error
	RetryCount     int            `json:"retry_count"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// SendRequest is the HTTP API payload for sending a message.
type SendRequest struct {
	Channel        ChannelType       `json:"channel"`
	Recipient      string            `json:"recipient"`
	RecipientID    string            `json:"recipient_id,omitempty"`
	Subject        string            `json:"subject,omitempty"`
	Body           string            `json:"body"`
	HTMLBody       string            `json:"html_body,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	TemplateID     string            `json:"template_id,omitempty"`
}

// SendResult is returned after a send attempt.
type SendResult struct {
	LogID       string         `json:"log_id"`
	Channel     ChannelType    `json:"channel"`
	Status      DeliveryStatus `json:"status"`
	ProviderRef string         `json:"provider_ref,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// ListFilter for querying delivery logs.
type ListFilter struct {
	Channel     ChannelType
	RecipientID string
	Status      DeliveryStatus
	Limit       int
	Offset      int
}
