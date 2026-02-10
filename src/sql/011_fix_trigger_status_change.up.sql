-- Fix trigger to not fire on moderation status-only changes
-- This prevents duplicate history entries when approving/rejecting versions

DROP TRIGGER IF EXISTS trigger_record_audit ON records;
DROP FUNCTION IF EXISTS record_audit_trigger();

-- Recreate the function to only fire on content changes (not status-only changes)
CREATE OR REPLACE FUNCTION record_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    next_version INTEGER;
BEGIN
    -- Only create history entry if actual content changed (not just status or modified_at)
    -- Check if s3_key, name, sha256, or metadata changed
    IF (OLD.s3_key IS DISTINCT FROM NEW.s3_key OR
        OLD.name IS DISTINCT FROM NEW.name OR
        OLD.sha256 IS DISTINCT FROM NEW.sha256 OR
        OLD.metadata IS DISTINCT FROM NEW.metadata) THEN
        
        -- Get next version number for this record
        SELECT COALESCE(MAX(version), 0) + 1 INTO next_version
        FROM record_history
        WHERE record_id = OLD.id;

        -- Insert the OLD values into history before update
        INSERT INTO record_history (
            record_id, version, s3_key, name, sha256, metadata,
            uploader_name, uploader_orcid, download_count,
            created_at, modified_at, moderation_status, change_type
        ) VALUES (
            OLD.id, next_version, OLD.s3_key, OLD.name, OLD.sha256, OLD.metadata,
            OLD.uploader_name, OLD.uploader_orcid, OLD.download_count,
            OLD.created_at, OLD.modified_at, OLD.moderation_status, TG_OP
        );
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate the trigger
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();

COMMENT ON FUNCTION record_audit_trigger() IS 'Archives record to history only when content changes (s3_key, name, sha256, or metadata), not on status-only updates';
