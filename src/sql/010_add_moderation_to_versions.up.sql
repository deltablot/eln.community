-- Add moderation_status to record_history table
ALTER TABLE record_history ADD COLUMN moderation_status VARCHAR(20) DEFAULT 'approved';

-- Update existing history records to have approved status (backward compatibility)
UPDATE record_history SET moderation_status = 'approved' WHERE moderation_status IS NULL;

-- Drop the old trigger and function
DROP TRIGGER IF EXISTS trigger_record_audit ON records;
DROP FUNCTION IF EXISTS record_audit_trigger();

-- Recreate the function to include moderation_status
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

-- Drop and recreate the delete trigger function to include moderation_status
DROP TRIGGER IF EXISTS trigger_record_delete_audit ON records;
DROP FUNCTION IF EXISTS record_delete_audit_trigger();

CREATE OR REPLACE FUNCTION record_delete_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    next_version INTEGER;
BEGIN
    SELECT COALESCE(MAX(version), 0) + 1 INTO next_version
    FROM record_history
    WHERE record_id = OLD.id;

    INSERT INTO record_history (
        record_id, version, s3_key, name, sha256, metadata,
        uploader_name, uploader_orcid, download_count,
        created_at, modified_at, moderation_status, change_type
    ) VALUES (
        OLD.id, next_version, OLD.s3_key, OLD.name, OLD.sha256, OLD.metadata,
        OLD.uploader_name, OLD.uploader_orcid, OLD.download_count,
        OLD.created_at, OLD.modified_at, OLD.moderation_status, 'DELETE'
    );

    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_record_delete_audit
BEFORE DELETE ON records
FOR EACH ROW
EXECUTE FUNCTION record_delete_audit_trigger();

-- Add comment
COMMENT ON COLUMN record_history.moderation_status IS 'Moderation status of this version when it was archived';
