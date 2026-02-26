-- Add archive (soft delete) columns to records
ALTER TABLE records ADD COLUMN archived_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE records ADD COLUMN archive_reason TEXT DEFAULT NULL;

-- Index for filtering active vs archived records
CREATE INDEX idx_records_archived_at ON records (archived_at);
