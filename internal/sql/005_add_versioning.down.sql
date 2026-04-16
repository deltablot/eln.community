-- Remove audit triggers and table
DROP TRIGGER IF EXISTS trigger_record_delete_audit ON records;
DROP TRIGGER IF EXISTS trigger_record_audit ON records;
DROP FUNCTION IF EXISTS record_delete_audit_trigger();
DROP FUNCTION IF EXISTS record_audit_trigger();

DROP INDEX IF EXISTS idx_record_history_archived_at;
DROP INDEX IF EXISTS idx_record_history_version;
DROP INDEX IF EXISTS idx_record_history_record_id;

DROP TABLE IF EXISTS record_history;
