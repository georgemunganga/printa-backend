package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service defines the routing engine business logic.
type Service interface {
	// RouteOrder evaluates all active rules and assigns the best store to an order.
	// It persists an immutable RoutingDecision and returns it.
	RouteOrder(ctx context.Context, req RouteOrderRequest) (*RoutingDecision, error)

	// GetDecision retrieves the latest routing decision for an order.
	GetDecision(ctx context.Context, orderID string) (*RoutingDecision, error)

	// OverrideRoute allows a human operator to manually reassign an order to a specific store.
	OverrideRoute(ctx context.Context, orderID string, req OverrideRouteRequest) (*RoutingDecision, error)

	// ListStoreDecisions returns all routing decisions assigned to a store.
	ListStoreDecisions(ctx context.Context, storeID string) ([]*RoutingDecision, error)

	// Rules management
	CreateRule(ctx context.Context, req CreateRuleRequest) (*RoutingRule, error)
	ListRules(ctx context.Context) ([]*RoutingRule, error)
	UpdateRule(ctx context.Context, id string, req CreateRuleRequest) (*RoutingRule, error)
	DeleteRule(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service { return &service{repo: repo} }

// ── Core Routing Engine ───────────────────────────────────────────────────────

// RouteOrder is the heart of the deterministic routing engine.
// It works in three stages:
//  1. Fetch all store candidates that carry the required products
//  2. Score each candidate against all active routing rules
//  3. Select the highest-scoring candidate and persist the decision
func (s *service) RouteOrder(ctx context.Context, req RouteOrderRequest) (*RoutingDecision, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}

	// Stage 1: Get eligible store candidates
	candidates, err := s.repo.GetStoreCandidates(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch store candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no eligible stores found for order %s — ensure products are stocked in at least one store", req.OrderID)
	}

	// Stage 2: Load active rules and score each candidate
	rules, err := s.repo.ListActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load routing rules: %w", err)
	}

	for _, candidate := range candidates {
		candidate.Score, candidate.Reason = s.scoreCandidate(candidate, rules)
	}

	// Stage 3: Select the best candidate (highest score; tie-break by fewest active jobs)
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Score > best.Score || (c.Score == best.Score && c.ActiveJobs < best.ActiveJobs) {
			best = c
		}
	}

	// Determine which rule drove the decision
	var appliedRule *RoutingRule
	for _, r := range rules {
		if r.TargetStoreID != nil && *r.TargetStoreID == best.StoreID {
			appliedRule = r
			break
		}
	}

	// Build and persist the decision
	decision := &RoutingDecision{
		ID:              uuid.New(),
		OrderID:         uuid.MustParse(req.OrderID),
		AssignedStoreID: best.StoreID,
		Reason:          best.Reason,
		Score:           best.Score,
		Status:          DecisionAssigned,
	}
	if appliedRule != nil {
		decision.RuleID = &appliedRule.ID
		decision.RuleName = appliedRule.Name
	}

	if err := s.repo.CreateDecision(ctx, decision); err != nil {
		return nil, fmt.Errorf("failed to persist routing decision: %w", err)
	}
	return decision, nil
}

// scoreCandidate computes a composite routing score for a store candidate.
// Scoring factors (each contributes up to 100 points):
//   - Product availability:  +100 if store stocks all required products
//   - Load balance:          +100 if queue is empty, decreasing by 10 per active job (min 0)
//   - Rule-based bonus:      +50 per matching TIER_PRIORITY or PRODUCT_CAPABILITY rule
func (s *service) scoreCandidate(c *StoreCandidate, rules []*RoutingRule) (float64, string) {
	var score float64
	var reasons []string

	// Factor 1: Product availability
	if c.HasProduct {
		score += 100
		reasons = append(reasons, "stocks required products")
	}

	// Factor 2: Load balancing — penalise busy queues
	loadScore := 100.0 - float64(c.ActiveJobs)*10.0
	if loadScore < 0 {
		loadScore = 0
	}
	score += loadScore
	reasons = append(reasons, fmt.Sprintf("%d active jobs (load score %.0f)", c.ActiveJobs, loadScore))

	// Factor 3: Rule-based bonuses
	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}
		// If the rule targets this specific store, apply a priority bonus
		if rule.TargetStoreID != nil && *rule.TargetStoreID == c.StoreID {
			bonus := float64(200 - rule.Priority) // higher priority rules give bigger bonus
			if bonus < 0 {
				bonus = 0
			}
			score += bonus
			reasons = append(reasons, fmt.Sprintf("rule '%s' bonus +%.0f", rule.Name, bonus))
		}

		// Apply LOAD_BALANCE rule: if queue depth is below threshold in conditions
		if rule.RuleType == RuleTypeLoadBalance {
			var conds map[string]interface{}
			if err := json.Unmarshal(rule.Conditions, &conds); err == nil {
				if maxJobs, ok := conds["max_active_jobs"].(float64); ok && float64(c.ActiveJobs) <= maxJobs {
					score += 25
					reasons = append(reasons, fmt.Sprintf("within load threshold (max %.0f jobs)", maxJobs))
				}
			}
		}
	}

	return score, strings.Join(reasons, "; ")
}

// ── Other Service Methods ─────────────────────────────────────────────────────

func (s *service) GetDecision(ctx context.Context, orderID string) (*RoutingDecision, error) {
	return s.repo.GetDecisionByOrderID(ctx, orderID)
}

func (s *service) OverrideRoute(ctx context.Context, orderID string, req OverrideRouteRequest) (*RoutingDecision, error) {
	if req.StoreID == "" {
		return nil, fmt.Errorf("store_id is required for override")
	}
	if req.Reason == "" {
		return nil, fmt.Errorf("reason is required for manual override")
	}

	storeUID, err := uuid.Parse(req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid store_id: %w", err)
	}
	orderUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order_id: %w", err)
	}

	// Mark the previous decision as overridden
	existing, err := s.repo.GetDecisionByOrderID(ctx, orderID)
	if err == nil && existing != nil {
		_ = s.repo.UpdateDecisionStatus(ctx, existing.ID.String(), DecisionOverridden)
	}

	// Create the new manual decision
	decision := &RoutingDecision{
		ID:              uuid.New(),
		OrderID:         orderUID,
		AssignedStoreID: storeUID,
		Reason:          fmt.Sprintf("Manual override: %s", req.Reason),
		Score:           0,
		Status:          DecisionAssigned,
	}
	if err := s.repo.CreateDecision(ctx, decision); err != nil {
		return nil, err
	}
	return decision, nil
}

func (s *service) ListStoreDecisions(ctx context.Context, storeID string) ([]*RoutingDecision, error) {
	return s.repo.ListDecisionsByStore(ctx, storeID)
}

func (s *service) CreateRule(ctx context.Context, req CreateRuleRequest) (*RoutingRule, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	if req.RuleType == "" {
		return nil, fmt.Errorf("rule_type is required")
	}
	if req.Priority <= 0 {
		req.Priority = 100
	}
	conditions := req.Conditions
	if len(conditions) == 0 {
		conditions = json.RawMessage(`{}`)
	}

	rule := &RoutingRule{
		ID:         uuid.New(),
		Name:       req.Name,
		Description: req.Description,
		RuleType:   RuleType(strings.ToUpper(req.RuleType)),
		Priority:   req.Priority,
		IsActive:   true,
		Conditions: conditions,
	}
	if req.TargetStoreID != "" {
		uid, err := uuid.Parse(req.TargetStoreID)
		if err != nil {
			return nil, fmt.Errorf("invalid target_store_id: %w", err)
		}
		rule.TargetStoreID = &uid
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *service) ListRules(ctx context.Context) ([]*RoutingRule, error) {
	return s.repo.ListActiveRules(ctx)
}

func (s *service) UpdateRule(ctx context.Context, id string, req CreateRuleRequest) (*RoutingRule, error) {
	rule, err := s.repo.GetRuleByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.RuleType != "" {
		rule.RuleType = RuleType(strings.ToUpper(req.RuleType))
	}
	if req.Priority > 0 {
		rule.Priority = req.Priority
	}
	if len(req.Conditions) > 0 {
		rule.Conditions = req.Conditions
	}
	if req.TargetStoreID != "" {
		uid, err := uuid.Parse(req.TargetStoreID)
		if err != nil {
			return nil, fmt.Errorf("invalid target_store_id: %w", err)
		}
		rule.TargetStoreID = &uid
	}
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *service) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}
