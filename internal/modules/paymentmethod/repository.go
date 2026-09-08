package paymentmethod

import "context"

type Repository interface {
	List(ctx context.Context, customerID string) ([]*Method, error)
	Get(ctx context.Context, id, customerID string) (*Method, error)
	Create(ctx context.Context, method *Method) error
	Update(ctx context.Context, method *Method) error
	Delete(ctx context.Context, id, customerID string) error
	SetDefault(ctx context.Context, id, customerID string) error
	PromoteEarliest(ctx context.Context, customerID string) error
}
