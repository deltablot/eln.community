-- Rollback moderation feature
DROP TABLE IF EXISTS moderation_actions;
DROP INDEX IF EXISTS idx_records_moderation_status;
ALTER TABLE records DROP COLUMN IF EXISTS moderation_status;
