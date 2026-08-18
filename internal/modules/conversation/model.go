package conversation

import (
	"time"

	"github.com/google/uuid"
)

// Message is a durable in-app text message belonging to an order conversation.
type Message struct {
	ID          uuid.UUID  `json:"id"`
	OrderID     uuid.UUID  `json:"order_id"`
	SenderID    uuid.UUID  `json:"sender_id"`
	Body        string     `json:"body"`
	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt time.Time  `json:"delivered_at"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}

// SendMessageRequest creates a text-only in-app message. File attachment support is
// intentionally deferred until it can reuse the customer-owned asset lifecycle safely.
type SendMessageRequest struct {
	Body string `json:"body"`
}
