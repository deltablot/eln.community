-- Add license field to records table
ALTER TABLE records ADD COLUMN IF NOT EXISTS license VARCHAR(255) NOT NULL DEFAULT 'CC-BY-4.0';

-- Add license field to record_history table
ALTER TABLE record_history ADD COLUMN IF NOT EXISTS license VARCHAR(255) NOT NULL DEFAULT 'CC-BY-4.0';

-- Add comment to explain the field
COMMENT ON COLUMN records.license IS 'License under which the record is shared (e.g., CC-BY-4.0)';
COMMENT ON COLUMN record_history.license IS 'License under which the record version was shared';
