CREATE UNIQUE INDEX IF NOT EXISTS ux_pos_transactions_completed_order
    ON pos_transactions (order_id)
    WHERE status = 'COMPLETED';
