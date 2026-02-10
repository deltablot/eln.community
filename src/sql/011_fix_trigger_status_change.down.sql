-- Revert to the old trigger that fires on all updates

DROP TRIGGER IF EXISTS trigger_record_audit ON records;
DROP FUNCTION IF EXISTS record_audit_trigger();

-- Recreate the old function that fires on all updates
CREATE OR REPLACE FUNCTION record_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    next_version INTEGER;
BEGIN
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

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate the trigger
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();
