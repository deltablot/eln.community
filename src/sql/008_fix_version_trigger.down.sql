-- Revert to original trigger that creates history on any UPDATE

DROP TRIGGER IF EXISTS trigger_record_audit ON records;

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
        created_at, modified_at, change_type
    ) VALUES (
        OLD.id, next_version, OLD.s3_key, OLD.name, OLD.sha256, OLD.metadata,
        OLD.uploader_name, OLD.uploader_orcid, OLD.download_count,
        OLD.created_at, OLD.modified_at, TG_OP
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate trigger
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();
