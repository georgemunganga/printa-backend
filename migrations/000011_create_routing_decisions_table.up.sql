-- routing_decisions: immutable audit log of every routing decision made for an order.
CREATE TABLE IF NOT EXISTS routing_decisions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    assigned_store_id UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    rule_id         UUID REFERENCES routing_rules(id) ON DELETE SET NULL,
    rule_name       VARCHAR(128),           -- snapshot of rule name at decision time
    reason          TEXT,                   -- human-readable explanation of why this store was chosen
    score           NUMERIC(8,4),           -- routing score (higher = better match)
    status          VARCHAR(32) NOT NULL DEFAULT 'ASSIGNED',  -- ASSIGNED, OVERRIDDEN, FAILED
    decided_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_decisions_order_id ON routing_decisions(order_id);
CREATE INDEX idx_routing_decisions_store_id ON routing_decisions(assigned_store_id);
