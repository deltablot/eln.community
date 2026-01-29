-- Fix the version trigger to only create history for meaningful changes
-- Don't create versions for download_count or moderation_status changes

DROP TRIGGER IF EXISTS trigger_record_audit ON records;

CREATE OR REPLACE FUNCTION record_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    next_version INTEGER;
BEGIN
    -- Only create history if meaningful fields changed
    -- Ignore changes to download_count and moderation_status
    IF (OLD.name IS DISTINCT FROM NEW.name) OR
       (OLD.sha256 IS DISTINCT FROM NEW.sha256) OR
       (OLD.metadata IS DISTINCT FROM NEW.metadata) OR
       (OLD.s3_key IS DISTINCT FROM NEW.s3_key) THEN
        
        -- Get next version number for this record
        SELECT COALESCE(MAX(version), 0) + 1 INTO next_version
        FROM record_history
        WHERE record_id = OLD.id;

        -- Insert the OLD values into history before update
        INSERT INTO record_history (
            record_id, version, s3_key, name, sha256, metadata,
            uploader_name, uploader_orcid, download_count,
            created_at, modified_at, change_type
        ) VALUES (
            OLD.id, next_version, OLD.s3_key, OLD.name, OLD.sha256, OLD.metadata,
            OLD.uploader_name, OLD.uploader_orcid, OLD.download_count,
            OLD.created_at, OLD.modified_at, TG_OP
        );
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate trigger
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();

COMMENT ON FUNCTION record_audit_trigger() IS 'Creates version history only for meaningful content changes (name, sha256, metadata, s3_key), ignoring download_count and moderation_status updates';
