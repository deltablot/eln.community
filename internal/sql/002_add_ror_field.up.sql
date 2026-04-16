-- Add ROR (Research Organization Registry) support with many-to-many relationship
-- ROR IDs follow the format: https://ror.org/0abcdef12

-- Create domain for ROR ID validation
CREATE DOMAIN rorid_type AS VARCHAR(9)
    CHECK (VALUE ~ '^0[a-z|0-9]{8}$');

-- Create records_ror table for many-to-many relationship
CREATE TABLE IF NOT EXISTS records_ror (
    record_id   UUID         NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    ror         rorid_type   NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (record_id, ror)
);

-- Add indexes for efficient lookups
CREATE INDEX idx_records_ror_record_id ON records_ror (record_id);
CREATE INDEX idx_records_ror_ror ON records_ror (ror);

-- Add comment to document the table
COMMENT ON TABLE records_ror IS 'Many-to-many relationship between records and ROR (Research Organization Registry) identifiers';
