-- Remove email column from admin_orcids table
DROP INDEX IF EXISTS idx_admin_orcids_email;
ALTER TABLE admin_orcids DROP COLUMN IF EXISTS email;
