package conversation

import (
	"time"

	"github.com/google/uuid"
)

// Attachment is a durable reference to a sender-owned design asset that is shared
// only through an authorized order conversation.
type Attachment struct {
	AssetID     uuid.UUID `json:"asset_id"`
	OwnerID     uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	URL         string    `json:"url,omitempty"`
}

// Message is a durable in-app message belonging to an order conversation.
type Message struct {
	ID          uuid.UUID     `json:"id"`
	OrderID     uuid.UUID     `json:"order_id"`
	SenderID    uuid.UUID     `json:"sender_id"`
	Body        string        `json:"body"`
	Attachments []*Attachment `json:"attachments,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	DeliveredAt time.Time     `json:"delivered_at"`
	ReadAt      *time.Time    `json:"read_at,omitempty"`
}

// SendMessageRequest creates a text message, an attachment message, or both.
// Asset IDs must refer to undeleted assets owned by the authenticated sender.
type SendMessageRequest struct {
	Body     string   `json:"body"`
	AssetIDs []string `json:"asset_ids,omitempty"`
}
