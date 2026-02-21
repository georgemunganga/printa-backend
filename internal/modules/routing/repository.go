package routing

import "context"

// Repository defines data access for routing rules and decisions.
type Repository interface {
	// Rules
	CreateRule(ctx context.Context, rule *RoutingRule) error
	GetRuleByID(ctx context.Context, id string) (*RoutingRule, error)
	ListActiveRules(ctx context.Context) ([]*RoutingRule, error)
	UpdateRule(ctx context.Context, rule *RoutingRule) error
	DeleteRule(ctx context.Context, id string) error

	// Decisions
	CreateDecision(ctx context.Context, d *RoutingDecision) error
	GetDecisionByOrderID(ctx context.Context, orderID string) (*RoutingDecision, error)
	ListDecisionsByStore(ctx context.Context, storeID string) ([]*RoutingDecision, error)
	UpdateDecisionStatus(ctx context.Context, id string, status DecisionStatus) error

	// Routing data helpers
	// GetStoreCandidates returns all stores that carry at least one product from the order,
	// along with their current active job count for load-balancing.
	GetStoreCandidates(ctx context.Context, orderID string) ([]*StoreCandidate, error)
}
