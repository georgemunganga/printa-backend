package payment

import (
	"time"

	"github.com/google/uuid"
)

// Provider represents a supported payment gateway.
type Provider string

const (
	ProviderMTNMomo    Provider = "MTN_MOMO"
	ProviderAirtel     Provider = "AIRTEL_MONEY"
	ProviderCash       Provider = "CASH"
	ProviderCard       Provider = "CARD"
)

// ReferenceType indicates what entity the payment is for.
type ReferenceType string

const (
	RefOrder        ReferenceType = "ORDER"
	RefInvoice      ReferenceType = "INVOICE"
	RefSubscription ReferenceType = "SUBSCRIPTION"
)

// TxStatus represents the internal lifecycle of a payment transaction.
type TxStatus string

const (
	TxPending    TxStatus = "PENDING"
	TxProcessing TxStatus = "PROCESSING"
	TxCompleted  TxStatus = "COMPLETED"
	TxFailed     TxStatus = "FAILED"
	TxCancelled  TxStatus = "CANCELLED"
	TxRefunded   TxStatus = "REFUNDED"
)

// PaymentTransaction is the provider-agnostic record of a payment attempt.
type PaymentTransaction struct {
	ID               uuid.UUID     `json:"id"`
	ReferenceType    ReferenceType `json:"reference_type"`
	ReferenceID      uuid.UUID     `json:"reference_id"`
	VendorID         *uuid.UUID    `json:"vendor_id,omitempty"`
	Provider         Provider      `json:"provider"`
	ProviderRef      string        `json:"provider_ref,omitempty"`
	ProviderStatus   string        `json:"provider_status,omitempty"`
	Status           TxStatus      `json:"status"`
	Amount           float64       `json:"amount"`
	Currency         string        `json:"currency"`
	PhoneNumber      string        `json:"phone_number,omitempty"`
	Description      string        `json:"description,omitempty"`
	WebhookReceivedAt *time.Time   `json:"webhook_received_at,omitempty"`
	WebhookPayload   interface{}   `json:"webhook_payload,omitempty"`
	IdempotencyKey   string        `json:"idempotency_key,omitempty"`
	RetryCount       int           `json:"retry_count"`
	LastError        string        `json:"last_error,omitempty"`
	Metadata         interface{}   `json:"metadata,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// ── Request/Response DTOs ─────────────────────────────────────────────────────

// InitiatePaymentRequest is the payload to start a new payment.
type InitiatePaymentRequest struct {
	Provider      string  `json:"provider"`       // MTN_MOMO | AIRTEL_MONEY | CASH | CARD
	ReferenceType string  `json:"reference_type"` // ORDER | INVOICE | SUBSCRIPTION
	ReferenceID   string  `json:"reference_id"`
	VendorID      string  `json:"vendor_id,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency,omitempty"` // defaults to ZMW
	PhoneNumber   string  `json:"phone_number,omitempty"`
	Description   string  `json:"description,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// WebhookPayload is the generic inbound webhook from a payment provider.
type WebhookPayload struct {
	Provider        string                 `json:"provider"`
	ExternalRef     string                 `json:"external_ref"`      // provider's transaction ID
	Status          string                 `json:"status"`            // provider-specific status string
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	PhoneNumber     string                 `json:"phone_number,omitempty"`
	RawPayload      map[string]interface{} `json:"raw_payload"`
}

// ProviderInitResponse is what a gateway adapter returns after initiating a payment.
type ProviderInitResponse struct {
	ProviderRef    string `json:"provider_ref"`    // external transaction ID
	ProviderStatus string `json:"provider_status"` // initial status from provider
	Message        string `json:"message,omitempty"`
}
