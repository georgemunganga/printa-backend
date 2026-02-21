package pos

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethod represents how a POS transaction was paid.
type PaymentMethod string

const (
	PaymentCash        PaymentMethod = "CASH"
	PaymentCard        PaymentMethod = "CARD"
	PaymentMobileMoney PaymentMethod = "MOBILE_MONEY"
	PaymentVoucher     PaymentMethod = "VOUCHER"
)

// TxStatus represents the state of a POS transaction.
type TxStatus string

const (
	TxPending   TxStatus = "PENDING"
	TxCompleted TxStatus = "COMPLETED"
	TxRefunded  TxStatus = "REFUNDED"
	TxFailed    TxStatus = "FAILED"
)

// POSTransaction records a payment event at the counter.
type POSTransaction struct {
	ID            uuid.UUID     `json:"id"`
	OrderID       uuid.UUID     `json:"order_id"`
	StoreID       uuid.UUID     `json:"store_id"`
	CashierID     *uuid.UUID    `json:"cashier_id,omitempty"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	Reference     string        `json:"reference,omitempty"`
	Status        TxStatus      `json:"status"`
	ChangeGiven   float64       `json:"change_given"`
	Notes         string        `json:"notes,omitempty"`
	TransactedAt  time.Time     `json:"transacted_at"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// CreateTransactionRequest is the payload for recording a POS payment.
type CreateTransactionRequest struct {
	OrderID       string  `json:"order_id"`
	StoreID       string  `json:"store_id"`
	CashierID     string  `json:"cashier_id,omitempty"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	Reference     string  `json:"reference,omitempty"`
	ChangeGiven   float64 `json:"change_given,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// RefundRequest is the payload for refunding a POS transaction.
type RefundRequest struct {
	Reason string `json:"reason"`
}
