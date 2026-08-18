CREATE TABLE IF NOT EXISTS submission_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requester_role    TEXT NOT NULL CHECK (requester_role IN ('CUSTOMER', 'VENDOR')),
    submission_kind   TEXT NOT NULL CHECK (submission_kind IN ('SUPPORT', 'FEEDBACK')),
    topic             TEXT NOT NULL,
    subject           TEXT NOT NULL,
    message           TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'RESOLVED', 'CLOSED')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (char_length(trim(topic)) BETWEEN 2 AND 80),
    CHECK (char_length(trim(subject)) BETWEEN 2 AND 160),
    CHECK (char_length(trim(message)) BETWEEN 10 AND 5000)
);

CREATE INDEX IF NOT EXISTS idx_submission_records_requester
    ON submission_records(requester_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_submission_records_role_kind
    ON submission_records(requester_role, submission_kind, created_at DESC);
