-- Queue of email notifications to be sent for record and comment moderation events
CREATE TABLE IF NOT EXISTS email_queue (
    id BIGSERIAL PRIMARY KEY,
    record_id UUID NOT NULL REFERENCES records(id),
    comment_id BIGINT REFERENCES comments(id),
    recipient_orcid orcid_type NOT NULL,
    send_from TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    notification_type TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ DEFAULT NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_email_queue_record_id ON email_queue(record_id);
CREATE INDEX idx_email_queue_comment_id ON email_queue(comment_id);
CREATE INDEX idx_email_queue_status_created_at ON email_queue(status, created_at);
