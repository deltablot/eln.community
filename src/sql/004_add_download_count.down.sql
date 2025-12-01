-- Remove download_count column from records table
ALTER TABLE records DROP COLUMN IF EXISTS download_count;
