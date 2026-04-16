-- Add moderation status to records table
ALTER TABLE records ADD COLUMN moderation_status VARCHAR(20) DEFAULT 'approved';

-- Update existing records to be approved (for backward compatibility)
UPDATE records SET moderation_status = 'approved' WHERE moderation_status IS NULL;

CREATE INDEX idx_records_moderation_status ON records(moderation_status);

-- Moderation actions log
CREATE TABLE moderation_actions (
    id BIGSERIAL PRIMARY KEY,
    record_id UUID NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    admin_orcid orcid_type NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'approve', 'reject', 'flag'
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX idx_moderation_actions_record ON moderation_actions(record_id);
CREATE INDEX idx_moderation_actions_admin ON moderation_actions(admin_orcid);
