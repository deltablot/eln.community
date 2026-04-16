-- Version-based moderation system
-- This migration consolidates all changes needed for version-based moderation:
-- 1. Add moderation_status to record_history
-- 2. Update trigger to only fire on content changes (not status-only)
-- 3. Add skip flag to prevent duplicate entries when approving pending versions
-- 4. Add version_name to moderation_actions for history display

-- Step 1: Add moderation_status to record_history table
ALTER TABLE record_history ADD COLUMN IF NOT EXISTS moderation_status VARCHAR(20) DEFAULT 'approved';

-- Update existing history records to have approved status (backward compatibility)
UPDATE record_history SET moderation_status = 'approved' WHERE moderation_status IS NULL;

-- Step 2: Add version_name to moderation_actions table
ALTER TABLE moderation_actions ADD COLUMN IF NOT EXISTS version_name VARCHAR(255);

-- Add comments
COMMENT ON COLUMN record_history.moderation_status IS 'Moderation status of this version when it was archived';
COMMENT ON COLUMN moderation_actions.version_name IS 'Name of the version that was moderated (for history display)';

-- Step 3: Drop and recreate triggers with all improvements
DROP TRIGGER IF EXISTS trigger_record_audit ON records;
DROP TRIGGER IF EXISTS trigger_record_delete_audit ON records;
DROP FUNCTION IF EXISTS record_audit_trigger();
DROP FUNCTION IF EXISTS record_delete_audit_trigger();

-- Create the update trigger function with:
-- - moderation_status support
-- - content-only change detection (skip status-only updates)
-- - skip flag support (prevent duplicates when approving pending versions)
CREATE OR REPLACE FUNCTION record_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    next_version INTEGER;
    skip_audit TEXT;
BEGIN
    -- Check if we should skip the audit (set by ApprovePendingVersion)
    skip_audit := current_setting('app.skip_audit_trigger', true);
    IF skip_audit = 'true' THEN
        RETURN NEW;
    END IF;

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

-- Create the delete trigger function with moderation_status support
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

-- Create the triggers
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();

CREATE TRIGGER trigger_record_delete_audit
BEFORE DELETE ON records
FOR EACH ROW
EXECUTE FUNCTION record_delete_audit_trigger();

-- Add function comments
COMMENT ON FUNCTION record_audit_trigger() IS 'Archives record to history only when content changes (not status-only updates) and skip flag is not set';
COMMENT ON FUNCTION record_delete_audit_trigger() IS 'Archives record to history before deletion';
