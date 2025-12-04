-- Record history/audit table - stores snapshots on every update
CREATE TABLE IF NOT EXISTS record_history (
    history_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    record_id     UUID NOT NULL,
    version       INTEGER NOT NULL DEFAULT 1,
    s3_key        TEXT NOT NULL,
    name          VARCHAR(255) NOT NULL,
    sha256        TEXT NOT NULL,
    metadata      JSONB NOT NULL DEFAULT '{}'::JSONB,
    uploader_name VARCHAR(255) NOT NULL,
    uploader_orcid CHAR(19) NOT NULL,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL,
    modified_at   TIMESTAMPTZ NOT NULL,
    archived_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    change_type   VARCHAR(20) NOT NULL DEFAULT 'UPDATE'
);

-- Indexes for efficient queries
CREATE INDEX idx_record_history_record_id ON record_history(record_id);
CREATE INDEX idx_record_history_version ON record_history(record_id, version);
CREATE INDEX idx_record_history_archived_at ON record_history(archived_at);

-- Function to auto-insert history on UPDATE
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

-- Trigger fires BEFORE UPDATE to capture old state
CREATE TRIGGER trigger_record_audit
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION record_audit_trigger();

-- Optional: Also capture DELETE operations
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
        created_at, modified_at, change_type
    ) VALUES (
        OLD.id, next_version, OLD.s3_key, OLD.name, OLD.sha256, OLD.metadata,
        OLD.uploader_name, OLD.uploader_orcid, OLD.download_count,
        OLD.created_at, OLD.modified_at, 'DELETE'
    );

    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_record_delete_audit
BEFORE DELETE ON records
FOR EACH ROW
EXECUTE FUNCTION record_delete_audit_trigger();

-- Comments
COMMENT ON TABLE record_history IS 'Audit table storing historical versions of records';
COMMENT ON COLUMN record_history.version IS 'Version number, auto-incremented per record';
COMMENT ON COLUMN record_history.archived_at IS 'Timestamp when this version was archived';
COMMENT ON COLUMN record_history.change_type IS 'Type of change: UPDATE or DELETE';
