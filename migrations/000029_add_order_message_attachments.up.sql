-- Messages may be text-only, attachment-only, or include both. Attachments reuse authenticated
-- design asset storage and remain owned by the sender; conversation access is enforced at retrieval.
ALTER TABLE order_messages
    DROP CONSTRAINT IF EXISTS order_messages_body_check;

ALTER TABLE order_messages
    ADD CONSTRAINT order_messages_body_length_check
    CHECK (char_length(trim(body)) <= 5000);

CREATE TABLE IF NOT EXISTS order_message_attachments (
    message_id UUID NOT NULL REFERENCES order_messages(id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES design_assets(id) ON DELETE RESTRICT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (message_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_order_message_attachments_asset
    ON order_message_attachments(asset_id);
