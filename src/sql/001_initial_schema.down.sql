-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS records_categories CASCADE;
DROP TABLE IF EXISTS records CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS admin_orcids CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;

-- Drop functions
DROP FUNCTION IF EXISTS update_modified_at() CASCADE;

-- Drop domain
DROP DOMAIN IF EXISTS orcid_type CASCADE;