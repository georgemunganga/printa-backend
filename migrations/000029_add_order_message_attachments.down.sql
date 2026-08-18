DROP TABLE IF EXISTS order_message_attachments;

ALTER TABLE order_messages
    DROP CONSTRAINT IF EXISTS order_messages_body_length_check;

ALTER TABLE order_messages
    ADD CONSTRAINT order_messages_body_check
    CHECK (char_length(trim(body)) BETWEEN 1 AND 5000);
