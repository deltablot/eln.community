BEGIN;

-- Edit table moderation_actions
ALTER TABLE moderation_actions RENAME TO records_moderation_logs;
ALTER TABLE records_moderation_logs RENAME COLUMN action TO moderation_status;

-- The old column contains actions such as:
-- approve, reject, delete and flag.
-- We also accept status names to support existing data.
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status TYPE INTEGER
  USING CASE moderation_status
    WHEN 'pending' THEN 0
    WHEN 'pending_review' THEN 0
    WHEN 'approve' THEN 1
    WHEN 'approved' THEN 1
    WHEN 'reject' THEN 2
    WHEN 'rejected' THEN 2
    WHEN 'delete' THEN 3
    WHEN 'deleted' THEN 3
    WHEN 'flag' THEN 4
    WHEN 'flagged' THEN 4
  END;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records_moderation_logs ADD CONSTRAINT records_moderation_logs_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));

-- new_status, previous_status and modified_at are schema placeholders
-- reserved for the future record moderation refactor.
ALTER TABLE records_moderation_logs ADD new_status INTEGER CHECK (new_status IN (0, 1, 2, 3, 4));
ALTER TABLE records_moderation_logs ADD previous_status INTEGER CHECK (previous_status IN (0, 1, 2, 3, 4));
ALTER TABLE records_moderation_logs ADD modified_at TIMESTAMP WITH TIME ZONE;

ALTER INDEX idx_moderation_actions_record RENAME TO idx_records_moderation_logs_record;
ALTER INDEX idx_moderation_actions_admin RENAME TO idx_records_moderation_logs_admin;
ALTER SEQUENCE moderation_actions_id_seq RENAME TO records_moderation_logs_id_seq;
COMMENT ON COLUMN records_moderation_logs.moderation_status IS 'Moderation status: 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';

-- Edit table comment_moderation_actions
ALTER TABLE comment_moderation_actions RENAME TO comments_moderation_logs;
ALTER TABLE comments_moderation_logs RENAME COLUMN admin_orcid TO reporter_orcid;
ALTER TABLE comments_moderation_logs RENAME COLUMN action TO new_status;

ALTER TABLE comments_moderation_logs ALTER COLUMN new_status TYPE INTEGER
  USING CASE new_status
    WHEN 'pending' THEN 0
    WHEN 'pending_review' THEN 0
    WHEN 'approve' THEN 1
    WHEN 'approved' THEN 1
    WHEN 'reject' THEN 2
    WHEN 'rejected' THEN 2
    WHEN 'delete' THEN 3
    WHEN 'deleted' THEN 3
    WHEN 'flag' THEN 4
    WHEN 'flagged' THEN 4
  END;
ALTER TABLE comments_moderation_logs ALTER COLUMN new_status SET NOT NULL;
ALTER TABLE comments_moderation_logs ADD CONSTRAINT comments_moderation_logs_new_status_check CHECK (new_status IN (0, 1, 2, 3, 4));
-- Historical logs do not contain the previous status.
-- New logs created by the application will explicitly provide it.
ALTER TABLE comments_moderation_logs ADD previous_status INTEGER CHECK (previous_status IN (0, 1, 2, 3, 4));
ALTER TABLE comments_moderation_logs ADD modified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
ALTER INDEX idx_comment_moderation_actions_comment RENAME TO idx_comments_moderation_logs_comment;
ALTER INDEX idx_comment_moderation_actions_admin RENAME TO idx_comments_moderation_logs_reporter;
ALTER SEQUENCE comment_moderation_actions_id_seq RENAME TO comments_moderation_logs_id_seq;

COMMENT ON COLUMN comments_moderation_logs.previous_status IS 'Previous status: NULL for historical logs, otherwise 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';
COMMENT ON COLUMN comments_moderation_logs.new_status IS 'New status: 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';

-- Edit table comments
ALTER TABLE comments ALTER COLUMN moderation_status DROP DEFAULT;

ALTER TABLE comments ALTER COLUMN moderation_status TYPE INTEGER
  USING CASE moderation_status
    WHEN 'pending' THEN 0
    WHEN 'pending_review' THEN 0
    WHEN 'approve' THEN 1
    WHEN 'approved' THEN 1
    WHEN 'reject' THEN 2
    WHEN 'rejected' THEN 2
    WHEN 'delete' THEN 3
    WHEN 'deleted' THEN 3
    WHEN 'flag' THEN 4
    WHEN 'flagged' THEN 4
  END;

ALTER TABLE comments ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE comments ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE comments ADD CONSTRAINT comments_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));

COMMENT ON COLUMN comments.moderation_status IS 'Moderation status: 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';

-- Edit table records
ALTER TABLE records ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records ALTER COLUMN moderation_status TYPE INTEGER
  USING CASE moderation_status
    WHEN 'pending' THEN 0
    WHEN 'pending_review' THEN 0
    WHEN 'approve' THEN 1
    WHEN 'approved' THEN 1
    WHEN 'reject' THEN 2
    WHEN 'rejected' THEN 2
    WHEN 'delete' THEN 3
    WHEN 'deleted' THEN 3
    WHEN 'flag' THEN 4
    WHEN 'flagged' THEN 4
  END;
ALTER TABLE records ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE records ADD CONSTRAINT records_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));
COMMENT ON COLUMN records.moderation_status IS 'Moderation status: 0 = pending, 1 = approve, 2 = reject, 3 = delete, 4 = flag';

-- Edit table record_history
ALTER TABLE record_history RENAME TO records_revisions;
ALTER TABLE records_revisions ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records_revisions ALTER COLUMN moderation_status TYPE INTEGER
  USING CASE moderation_status
    WHEN 'pending' THEN 0
    WHEN 'pending_review' THEN 0
    WHEN 'approve' THEN 1
    WHEN 'approved' THEN 1
    WHEN 'reject' THEN 2
    WHEN 'rejected' THEN 2
    WHEN 'delete' THEN 3
    WHEN 'deleted' THEN 3
    WHEN 'flag' THEN 4
    WHEN 'flagged' THEN 4
  END;

ALTER TABLE records_revisions ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records_revisions ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE records_revisions ADD CONSTRAINT records_revisions_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));
COMMENT ON COLUMN records_revisions.moderation_status IS 'Moderation status: 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';

-- Existing trigger functions still refer to record_history.
-- This simple compatibility view allows them to continue working
-- without rewriting the trigger functions in this migration.
CREATE VIEW record_history AS
SELECT *
FROM records_revisions;
COMMIT;
