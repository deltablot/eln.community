-- Add ROR (Research Organization Registry) field to records table
-- ROR IDs follow the format: https://ror.org/0abcdef12
ALTER TABLE records ADD COLUMN ror_id VARCHAR(255);

-- Add index for ROR ID lookups
CREATE INDEX idx_records_ror_id ON records (ror_id);

-- Add comment to document the field
COMMENT ON COLUMN records.ror_id IS 'Research Organization Registry (ROR) identifier - https://ror.org/';
