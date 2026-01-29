-- Remove license field from records table
ALTER TABLE records DROP COLUMN IF EXISTS license;

-- Remove license field from record_history table
ALTER TABLE record_history DROP COLUMN IF EXISTS license;
