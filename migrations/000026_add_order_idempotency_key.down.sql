DROP INDEX IF EXISTS idx_orders_idempotency_key;
ALTER TABLE orders DROP COLUMN IF EXISTS idempotency_key;
