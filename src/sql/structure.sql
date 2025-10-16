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
  CHECK (VALUE ~ '^\d{4}-\d{4}-\d{4}-\d{3}[\dX]$');

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

-- RECORD_CATEGORIES (Many-to-Many relationship between records and categories)
CREATE TABLE IF NOT EXISTS record_categories (
  record_id   UUID         NOT NULL REFERENCES records(id) ON DELETE CASCADE,
  category_id INTEGER      NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
  PRIMARY KEY (record_id, category_id)
);
CREATE INDEX idx_record_categories_record_id ON record_categories (record_id);
CREATE INDEX idx_record_categories_category_id ON record_categories (category_id);
-- END RECORD_CATEGORIES

-- ADMIN_ORCIDS
CREATE TABLE IF NOT EXISTS admin_orcids
(
    orcid       orcid_type PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trigger_update_modified_at_admin_orcids
    BEFORE UPDATE
    ON admin_orcids
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_at();

