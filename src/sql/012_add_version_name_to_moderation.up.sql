-- Add version_name to moderation_actions to track the actual name of the version being moderated
ALTER TABLE moderation_actions ADD COLUMN version_name VARCHAR(255);

-- Add comment
COMMENT ON COLUMN moderation_actions.version_name IS 'Name of the version that was moderated (for history display)';
