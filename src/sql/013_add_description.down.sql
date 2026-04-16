-- Remove description column from records table
ALTER TABLE records DROP COLUMN IF EXISTS description;

 -- Remove description column to record_history table
ALTER TABLE record_history DROP COLUMN IF EXISTS description;
