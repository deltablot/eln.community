-- modified_at
CREATE OR REPLACE FUNCTION update_modified_at()
RETURNS TRIGGER AS $$
BEGIN
NEW.modified_at = now();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- orcid format
CREATE DOMAIN orcid_type AS CHAR(19)
  CHECK (VALUE ~ '^\d{4}-\d{4}-\d{4}-\d{4}$');

-- RECORDS
CREATE TABLE IF NOT EXISTS records (
  id         UUID                     PRIMARY KEY,
  s3_key     TEXT                     NOT NULL,
  name VARCHAR(255)               NOT NULL UNIQUE,
  sha256     TEXT                     UNIQUE NOT NULL,
  metadata   JSONB                    NOT NULL DEFAULT '{}'::JSONB,
  created_at TIMESTAMPTZ              NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ     NOT NULL DEFAULT now(),
  uploader_name VARCHAR(255)               NOT NULL,
  uploader_orcid   orcid_type  NOT NULL
);
CREATE TRIGGER trigger_update_modified_at
BEFORE UPDATE ON records
FOR EACH ROW
EXECUTE FUNCTION update_modified_at();
-- END RECORDS

-- CATEGORIES
CREATE TABLE IF NOT EXISTS categories (
  id         INTEGER       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name VARCHAR(255)               NOT NULL UNIQUE,
  created_at TIMESTAMPTZ              NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ     NOT NULL DEFAULT now()
);
-- modified_at
CREATE TRIGGER trigger_update_modified_at
BEFORE UPDATE ON categories
FOR EACH ROW
EXECUTE FUNCTION update_modified_at();

-- SESSIONS
CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BYTEA NOT NULL,
	expiry TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);
