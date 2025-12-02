-- Add download_count column to records table
ALTER TABLE records ADD COLUMN download_count INTEGER NOT NULL DEFAULT 0;
