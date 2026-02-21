package production

import "context"

// Repository defines data access for production jobs.
type Repository interface {
	Create(ctx context.Context, job *ProductionJob) error
	GetByID(ctx context.Context, id string) (*ProductionJob, error)
	GetByOrderID(ctx context.Context, orderID string) (*ProductionJob, error)
	ListByStore(ctx context.Context, storeID string, status string) ([]*ProductionJob, error)
	ListByAssignee(ctx context.Context, userID string) ([]*ProductionJob, error)
	UpdateStatus(ctx context.Context, id string, status JobStatus, notes string) error
	UpdateAssignee(ctx context.Context, id string, userID string) error
	CountActiveByStore(ctx context.Context, storeID string) (int, error)
}
