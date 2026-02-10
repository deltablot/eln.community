-- Remove version_name column from moderation_actions
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS version_name;
