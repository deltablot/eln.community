-- Comments table for user comments on records
CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    record_id UUID NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    commenter_name VARCHAR(255) NOT NULL,
    commenter_orcid orcid_type NOT NULL,
    content TEXT NOT NULL,
    moderation_status VARCHAR(20) NOT NULL DEFAULT 'pending_review',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_not_empty CHECK (LENGTH(TRIM(content)) > 0)
);

-- Indexes for efficient queries
CREATE INDEX idx_comments_record_id ON comments(record_id);
CREATE INDEX idx_comments_commenter_orcid ON comments(commenter_orcid);
CREATE INDEX idx_comments_moderation_status ON comments(moderation_status);
CREATE INDEX idx_comments_created_at ON comments(created_at DESC);

-- Trigger for modified_at
CREATE TRIGGER trigger_update_modified_at_comments
    BEFORE UPDATE ON comments
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_at();

-- Comment moderation actions log
CREATE TABLE IF NOT EXISTS comment_moderation_actions (
    id BIGSERIAL PRIMARY KEY,
    comment_id BIGINT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    admin_orcid orcid_type NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'approve', 'reject', 'delete'
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comment_moderation_actions_comment ON comment_moderation_actions(comment_id);
CREATE INDEX idx_comment_moderation_actions_admin ON comment_moderation_actions(admin_orcid);

-- Comments
COMMENT ON TABLE comments IS 'User comments on records with moderation support';
COMMENT ON COLUMN comments.content IS 'Raw text content only, no HTML allowed';
COMMENT ON COLUMN comments.moderation_status IS 'Moderation status: pending_review, approved, rejected';
