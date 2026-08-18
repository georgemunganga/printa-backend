package submission

import "context"

type Repository interface {
	Create(ctx context.Context, input CreateInput) (*Record, error)
	ListForRequester(ctx context.Context, requesterUserID string, requesterRole RequesterRole) ([]Record, error)
	ListForRole(ctx context.Context, requesterRole RequesterRole) ([]Record, error)
}
