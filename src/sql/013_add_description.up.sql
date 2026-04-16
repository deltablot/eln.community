-- Add description column to records table
ALTER TABLE records ADD COLUMN description TEXT;

 -- Add description column to record_history table
ALTER TABLE record_history ADD COLUMN description TEXT;
