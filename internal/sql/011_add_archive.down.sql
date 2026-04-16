-- Restore absolute unique constraints
DROP INDEX IF EXISTS records_name_active_key;
DROP INDEX IF EXISTS records_sha256_active_key;
ALTER TABLE records ADD CONSTRAINT records_sha256_key UNIQUE (sha256);
ALTER TABLE records ADD CONSTRAINT records_name_key UNIQUE (name);

-- Remove archive columns from records
DROP INDEX IF EXISTS idx_records_archived_at;
ALTER TABLE records DROP COLUMN IF EXISTS archive_reason;
ALTER TABLE records DROP COLUMN IF EXISTS archived_at;
