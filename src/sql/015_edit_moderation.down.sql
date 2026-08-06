BEGIN;

-- Remove the compatibility view before restoring the original table name.
DROP VIEW record_history;

-- Revert table records_revisions
ALTER TABLE records_revisions DROP CONSTRAINT records_revisions_moderation_status_check;
ALTER TABLE records_revisions ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records_revisions ALTER COLUMN moderation_status TYPE VARCHAR(20)
  USING CASE moderation_status
    WHEN 0 THEN 'pending_review'
    WHEN 1 THEN 'approved'
    WHEN 2 THEN 'rejected'
    WHEN 3 THEN 'deleted'
    WHEN 4 THEN 'flagged'
  END;
-- The original column was nullable
ALTER TABLE records_revisions ALTER COLUMN moderation_status DROP NOT NULL;
ALTER TABLE records_revisions ALTER COLUMN moderation_status SET DEFAULT 'approved';
ALTER TABLE records_revisions RENAME TO record_history;
COMMENT ON COLUMN record_history.moderation_status IS 'Moderation status of this version when it was archived';


-- Revert table records
ALTER TABLE records DROP CONSTRAINT records_moderation_status_check;
ALTER TABLE records ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records ALTER COLUMN moderation_status TYPE VARCHAR(20)
  USING CASE moderation_status
    WHEN 0 THEN 'pending_review'
    WHEN 1 THEN 'approved'
    WHEN 2 THEN 'rejected'
    WHEN 3 THEN 'deleted'
    WHEN 4 THEN 'flagged'
  END;

-- The original column was nullable
ALTER TABLE records ALTER COLUMN moderation_status DROP NOT NULL;
ALTER TABLE records ALTER COLUMN moderation_status SET DEFAULT 'approved';

-- Revert table comments
ALTER TABLE comments DROP CONSTRAINT comments_moderation_status_check;
ALTER TABLE comments ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE comments ALTER COLUMN moderation_status TYPE VARCHAR(20)
  USING CASE moderation_status
    WHEN 0 THEN 'pending_review'
    WHEN 1 THEN 'approved'
    WHEN 2 THEN 'rejected'
    WHEN 3 THEN 'deleted'
    WHEN 4 THEN 'flagged'
  END;
ALTER TABLE comments ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE comments ALTER COLUMN moderation_status SET DEFAULT 'pending_review';
COMMENT ON COLUMN comments.moderation_status IS 'Moderation status: pending_review, approved, rejected';

-- Revert table comments_moderation_logs
ALTER SEQUENCE comments_moderation_logs_id_seq RENAME TO comment_moderation_actions_id_seq;
ALTER INDEX idx_comments_moderation_logs_comment RENAME TO idx_comment_moderation_actions_comment;
ALTER INDEX idx_comments_moderation_logs_reporter RENAME TO idx_comment_moderation_actions_admin;

COMMENT ON COLUMN comments_moderation_logs.previous_status IS NULL;
COMMENT ON COLUMN comments_moderation_logs.new_status IS NULL;
ALTER TABLE comments_moderation_logs DROP COLUMN previous_status;
ALTER TABLE comments_moderation_logs DROP COLUMN modified_at;
ALTER TABLE comments_moderation_logs DROP CONSTRAINT comments_moderation_logs_new_status_check;
ALTER TABLE comments_moderation_logs ALTER COLUMN new_status TYPE TEXT
  USING CASE new_status
    WHEN 0 THEN 'pending'
    WHEN 1 THEN 'approve'
    WHEN 2 THEN 'reject'
    WHEN 3 THEN 'delete'
    WHEN 4 THEN 'flag'
  END;
ALTER TABLE comments_moderation_logs ALTER COLUMN new_status SET NOT NULL;
ALTER TABLE comments_moderation_logs RENAME COLUMN new_status TO action;
ALTER TABLE comments_moderation_logs RENAME COLUMN reporter_orcid TO admin_orcid;
ALTER TABLE comments_moderation_logs RENAME TO comment_moderation_actions;


-- Revert table records_moderation_logs
ALTER SEQUENCE records_moderation_logs_id_seq RENAME TO moderation_actions_id_seq;
ALTER INDEX idx_records_moderation_logs_record RENAME TO idx_moderation_actions_record;
ALTER INDEX idx_records_moderation_logs_admin RENAME TO idx_moderation_actions_admin;
ALTER TABLE records_moderation_logs DROP COLUMN new_status;
ALTER TABLE records_moderation_logs DROP COLUMN previous_status;
ALTER TABLE records_moderation_logs DROP COLUMN modified_at;
ALTER TABLE records_moderation_logs  DROP CONSTRAINT records_moderation_logs_moderation_status_check;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status TYPE VARCHAR(20)
  USING CASE moderation_status
    WHEN 0 THEN 'pending'
    WHEN 1 THEN 'approve'
    WHEN 2 THEN 'reject'
    WHEN 3 THEN 'delete'
    WHEN 4 THEN 'flag'
  END;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records_moderation_logs RENAME COLUMN moderation_status TO action;
ALTER TABLE records_moderation_logs RENAME TO moderation_actions;

COMMIT;
