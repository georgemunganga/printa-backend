-- production_jobs: tracks the print/production lifecycle for each order assigned to a store.
CREATE TABLE IF NOT EXISTS production_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    store_id        UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    assigned_to     UUID REFERENCES users(id) ON DELETE SET NULL,  -- operator/staff member
    status          VARCHAR(32) NOT NULL DEFAULT 'QUEUED',
    -- QUEUED | IN_PROGRESS | ON_HOLD | COMPLETED | CANCELLED
    priority        INT NOT NULL DEFAULT 5,   -- 1=URGENT, 5=NORMAL, 10=LOW
    notes           TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    due_at          TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_production_jobs_order_id  ON production_jobs(order_id);
CREATE INDEX idx_production_jobs_store_id  ON production_jobs(store_id);
CREATE INDEX idx_production_jobs_status    ON production_jobs(status);
CREATE INDEX idx_production_jobs_priority  ON production_jobs(priority ASC);
