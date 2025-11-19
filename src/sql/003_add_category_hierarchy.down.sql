-- Drop the trigger and function
DROP TRIGGER IF EXISTS trigger_check_category_depth ON categories;
DROP FUNCTION IF EXISTS check_category_depth();

-- Drop the index
DROP INDEX IF EXISTS idx_categories_parent_id;

-- Remove the parent_id column
ALTER TABLE categories DROP COLUMN IF EXISTS parent_id;
