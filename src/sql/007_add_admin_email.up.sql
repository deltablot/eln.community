-- Add email column to admin_orcids table
ALTER TABLE admin_orcids ADD COLUMN email VARCHAR(255);

-- Add index for email lookups
CREATE INDEX idx_admin_orcids_email ON admin_orcids(email);
