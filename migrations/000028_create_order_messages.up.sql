-- Customer and vendor/staff messages are scoped to a single order. Attachments are intentionally excluded
-- until an authenticated message-attachment lifecycle is implemented on top of design asset storage.
CREATE TABLE IF NOT EXISTS order_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    body TEXT NOT NULL CHECK (char_length(trim(body)) BETWEEN 1 AND 5000),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    read_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_order_messages_order_created
    ON order_messages(order_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_order_messages_sender_created
    ON order_messages(sender_id, created_at DESC);
