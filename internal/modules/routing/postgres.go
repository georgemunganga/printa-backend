package routing

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

// ── Rules ─────────────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateRule(ctx context.Context, rule *RoutingRule) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO routing_rules (id, name, description, rule_type, priority, is_active, conditions, target_store_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		rule.ID, rule.Name, rule.Description, rule.RuleType, rule.Priority,
		rule.IsActive, rule.Conditions, rule.TargetStoreID)
	return err
}

func (r *postgresRepo) GetRuleByID(ctx context.Context, id string) (*RoutingRule, error) {
	return r.scanRule(r.db.QueryRowContext(ctx, `
		SELECT id,name,description,rule_type,priority,is_active,conditions,target_store_id,created_at,updated_at
		FROM routing_rules WHERE id=$1`, id))
}

func (r *postgresRepo) ListActiveRules(ctx context.Context) ([]*RoutingRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,name,description,rule_type,priority,is_active,conditions,target_store_id,created_at,updated_at
		FROM routing_rules WHERE is_active=TRUE ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*RoutingRule
	for rows.Next() {
		rule, err := r.scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *postgresRepo) UpdateRule(ctx context.Context, rule *RoutingRule) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE routing_rules SET name=$1,description=$2,rule_type=$3,priority=$4,
		is_active=$5,conditions=$6,target_store_id=$7,updated_at=$8 WHERE id=$9`,
		rule.Name, rule.Description, rule.RuleType, rule.Priority,
		rule.IsActive, rule.Conditions, rule.TargetStoreID, time.Now(), rule.ID)
	return err
}

func (r *postgresRepo) DeleteRule(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM routing_rules WHERE id=$1`, id)
	return err
}

// ── Decisions ─────────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateDecision(ctx context.Context, d *RoutingDecision) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO routing_decisions (id,order_id,assigned_store_id,rule_id,rule_name,reason,score,status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		d.ID, d.OrderID, d.AssignedStoreID, d.RuleID, d.RuleName, d.Reason, d.Score, d.Status)
	return err
}

func (r *postgresRepo) GetDecisionByOrderID(ctx context.Context, orderID string) (*RoutingDecision, error) {
	return r.scanDecision(r.db.QueryRowContext(ctx, `
		SELECT id,order_id,assigned_store_id,rule_id,rule_name,reason,score,status,decided_at
		FROM routing_decisions WHERE order_id=$1 ORDER BY decided_at DESC LIMIT 1`, orderID))
}

func (r *postgresRepo) ListDecisionsByStore(ctx context.Context, storeID string) ([]*RoutingDecision, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,order_id,assigned_store_id,rule_id,rule_name,reason,score,status,decided_at
		FROM routing_decisions WHERE assigned_store_id=$1 ORDER BY decided_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var decisions []*RoutingDecision
	for rows.Next() {
		d, err := r.scanDecision(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

func (r *postgresRepo) UpdateDecisionStatus(ctx context.Context, id string, status DecisionStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE routing_decisions SET status=$1 WHERE id=$2`, status, id)
	return err
}

// GetStoreCandidates returns all stores that stock at least one product from the order.
// NOTE: production_jobs join is added in Phase 5 once that table exists.
func (r *postgresRepo) GetStoreCandidates(ctx context.Context, orderID string) ([]*StoreCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		    s.id,
		    s.name,
		    COUNT(DISTINCT vsp.id) AS product_matches,
		    0 AS active_jobs
		FROM stores s
		JOIN vendor_store_products vsp ON vsp.store_id = s.id AND vsp.is_available = TRUE
		JOIN order_items oi ON oi.vendor_store_product_id = vsp.id
		JOIN orders o ON o.id = oi.order_id AND o.id = $1
		GROUP BY s.id, s.name
		ORDER BY product_matches DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []*StoreCandidate
	for rows.Next() {
		c := &StoreCandidate{}
		var productMatches int
		if err := rows.Scan(&c.StoreID, &c.StoreName, &productMatches, &c.ActiveJobs); err != nil {
			return nil, err
		}
		c.HasProduct = productMatches > 0
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// ── scanners ──────────────────────────────────────────────────────────────────

type ruleScanner interface {
	Scan(dest ...interface{}) error
}

func (r *postgresRepo) scanRule(row ruleScanner) (*RoutingRule, error) {
	rule := &RoutingRule{}
	var targetStoreID sql.NullString
	var conditions []byte
	err := row.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.RuleType,
		&rule.Priority, &rule.IsActive, &conditions, &targetStoreID,
		&rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	rule.Conditions = conditions
	if targetStoreID.Valid {
		uid, _ := uuid.Parse(targetStoreID.String)
		rule.TargetStoreID = &uid
	}
	return rule, nil
}

type decisionScanner interface {
	Scan(dest ...interface{}) error
}

func (r *postgresRepo) scanDecision(row decisionScanner) (*RoutingDecision, error) {
	d := &RoutingDecision{}
	var ruleID sql.NullString
	err := row.Scan(&d.ID, &d.OrderID, &d.AssignedStoreID, &ruleID,
		&d.RuleName, &d.Reason, &d.Score, &d.Status, &d.DecidedAt)
	if err != nil {
		return nil, err
	}
	if ruleID.Valid {
		uid, _ := uuid.Parse(ruleID.String)
		d.RuleID = &uid
	}
	return d, nil
}
