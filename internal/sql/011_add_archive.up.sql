-- Add archive (soft delete) columns to records
ALTER TABLE records ADD COLUMN archived_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE records ADD COLUMN archive_reason TEXT DEFAULT NULL;

-- Index for filtering active vs archived records
CREATE INDEX idx_records_archived_at ON records (archived_at);

-- Replace absolute unique constraints with partial unique indexes
-- so that archived records don't block new uploads with the same sha256/name
ALTER TABLE records DROP CONSTRAINT records_sha256_key;
ALTER TABLE records DROP CONSTRAINT records_name_key;
CREATE UNIQUE INDEX records_sha256_active_key ON records (sha256) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX records_name_active_key ON records (name) WHERE archived_at IS NULL;
