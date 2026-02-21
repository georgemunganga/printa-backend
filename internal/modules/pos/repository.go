package pos

import "context"

// Repository defines data access for POS transactions.
type Repository interface {
	Create(ctx context.Context, tx *POSTransaction) error
	GetByID(ctx context.Context, id string) (*POSTransaction, error)
	GetByOrderID(ctx context.Context, orderID string) (*POSTransaction, error)
	ListByStore(ctx context.Context, storeID string) ([]*POSTransaction, error)
	UpdateStatus(ctx context.Context, id string, status TxStatus) error
}
