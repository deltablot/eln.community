-- Queue of email notifications to be sent for record and comment moderation events
CREATE TABLE IF NOT EXISTS email_queue (
    id INTEGER       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    record_id UUID NOT NULL REFERENCES records(id),
    comment_id BIGINT REFERENCES comments(id),
    recipient_orcid orcid_type NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ DEFAULT NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_email_queue_status_created_at ON email_queue(status);
