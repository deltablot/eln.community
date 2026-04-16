-- Add parent_id column to categories table to support nested categories (max 3 levels deep)
ALTER TABLE categories ADD COLUMN parent_id INTEGER REFERENCES categories(id) ON DELETE CASCADE;

-- Add index for parent_id lookups
CREATE INDEX idx_categories_parent_id ON categories (parent_id);

-- Add a check constraint to prevent more than 3 levels of nesting
-- This will be enforced at the application level as well
CREATE OR REPLACE FUNCTION check_category_depth()
RETURNS TRIGGER AS $$
DECLARE
    depth INTEGER := 0;
    current_parent_id INTEGER;
BEGIN
    -- If no parent, depth is 0 (root level)
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;
    
    -- Count the depth by traversing up the parent chain
    current_parent_id := NEW.parent_id;
    WHILE current_parent_id IS NOT NULL LOOP
        depth := depth + 1;
        IF depth >= 3 THEN
            RAISE EXCEPTION 'Categories can only be nested up to 3 levels deep';
        END IF;
        
        SELECT parent_id INTO current_parent_id
        FROM categories
        WHERE id = current_parent_id;
    END LOOP;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_check_category_depth
BEFORE INSERT OR UPDATE ON categories
FOR EACH ROW
EXECUTE FUNCTION check_category_depth();
