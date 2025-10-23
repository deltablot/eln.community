-- Remove ROR field from records table
DROP INDEX IF EXISTS idx_records_ror_id;
ALTER TABLE records DROP COLUMN IF EXISTS ror_id;