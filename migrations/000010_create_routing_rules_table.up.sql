-- routing_rules: configurable rules that govern how orders are routed to stores/queues.
-- Rules are evaluated in priority order (lower number = higher priority).
CREATE TABLE IF NOT EXISTS routing_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(128) NOT NULL,
    description     TEXT,
    rule_type       VARCHAR(32) NOT NULL,   -- PRODUCT_CAPABILITY, GEO_PROXIMITY, LOAD_BALANCE, TIER_PRIORITY
    priority        INT NOT NULL DEFAULT 100,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    conditions      JSONB NOT NULL DEFAULT '{}',  -- e.g. {"product_category": "FLYERS", "max_distance_km": 50}
    target_store_id UUID REFERENCES stores(id) ON DELETE SET NULL,  -- NULL = applies to all stores
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_rules_active   ON routing_rules(is_active);
CREATE INDEX idx_routing_rules_priority ON routing_rules(priority ASC);
CREATE INDEX idx_routing_rules_type     ON routing_rules(rule_type);
