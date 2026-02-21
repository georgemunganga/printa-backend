package routing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RuleType categorises the routing strategy a rule implements.
type RuleType string

const (
	RuleTypeProductCapability RuleType = "PRODUCT_CAPABILITY" // Route to stores that stock the required product
	RuleTypeGeoProximity      RuleType = "GEO_PROXIMITY"      // Route to the nearest store
	RuleTypeLoadBalance       RuleType = "LOAD_BALANCE"       // Route to the store with the fewest active jobs
	RuleTypeTierPriority      RuleType = "TIER_PRIORITY"      // Route to stores owned by higher-tier vendors first
)

// DecisionStatus tracks the outcome of a routing decision.
type DecisionStatus string

const (
	DecisionAssigned   DecisionStatus = "ASSIGNED"
	DecisionOverridden DecisionStatus = "OVERRIDDEN"
	DecisionFailed     DecisionStatus = "FAILED"
)

// RoutingRule is a configurable rule that governs order routing.
type RoutingRule struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	RuleType      RuleType        `json:"rule_type"`
	Priority      int             `json:"priority"`
	IsActive      bool            `json:"is_active"`
	Conditions    json.RawMessage `json:"conditions"`
	TargetStoreID *uuid.UUID      `json:"target_store_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// RoutingDecision is an immutable audit record of a routing outcome for an order.
type RoutingDecision struct {
	ID              uuid.UUID      `json:"id"`
	OrderID         uuid.UUID      `json:"order_id"`
	AssignedStoreID uuid.UUID      `json:"assigned_store_id"`
	RuleID          *uuid.UUID     `json:"rule_id,omitempty"`
	RuleName        string         `json:"rule_name,omitempty"`
	Reason          string         `json:"reason"`
	Score           float64        `json:"score"`
	Status          DecisionStatus `json:"status"`
	DecidedAt       time.Time      `json:"decided_at"`
}

// StoreCandidate represents a store being evaluated during routing with its computed score.
type StoreCandidate struct {
	StoreID     uuid.UUID `json:"store_id"`
	StoreName   string    `json:"store_name"`
	Score       float64   `json:"score"`
	Reason      string    `json:"reason"`
	ActiveJobs  int       `json:"active_jobs"`  // current production queue depth
	HasProduct  bool      `json:"has_product"`  // stocks all required products
}

// RouteOrderRequest is the payload to trigger routing for an order.
type RouteOrderRequest struct {
	OrderID string `json:"order_id"`
}

// OverrideRouteRequest allows a human operator to manually override a routing decision.
type OverrideRouteRequest struct {
	StoreID string `json:"store_id"`
	Reason  string `json:"reason"`
}

// CreateRuleRequest is the payload for creating a new routing rule.
type CreateRuleRequest struct {
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	RuleType      string          `json:"rule_type"`
	Priority      int             `json:"priority"`
	Conditions    json.RawMessage `json:"conditions"`
	TargetStoreID string          `json:"target_store_id,omitempty"`
}
