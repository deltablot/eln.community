-- Remove archive columns from records
DROP INDEX IF EXISTS idx_records_archived_at;
ALTER TABLE records DROP COLUMN IF EXISTS archive_reason;
ALTER TABLE records DROP COLUMN IF EXISTS archived_at;
