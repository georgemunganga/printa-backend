package notification

import "context"

// Repository defines all data access operations for the notifications module.
type Repository interface {
	Create(ctx context.Context, n *Notification) error
	// CreateWithOutbox persists the notification and external-delivery event in one transaction.
	CreateWithOutbox(ctx context.Context, n *Notification, eventType string, payload []byte) error
	BulkCreate(ctx context.Context, notifications []*Notification) error
	GetByID(ctx context.Context, id string) (*Notification, error)
	List(ctx context.Context, filter ListFilter) ([]*Notification, int, error)
	MarkRead(ctx context.Context, id, recipientID string) error
	MarkAllRead(ctx context.Context, recipientID string) error
	Dismiss(ctx context.Context, id, recipientID string) error
	Delete(ctx context.Context, id, recipientID string) error
	GetUnreadCount(ctx context.Context, recipientID string) (int, error)
}
