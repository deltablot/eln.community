-- Edit table moderation_actions
ALTER TABLE moderation_actions RENAME TO records_moderation_logs;
ALTER TABLE records_moderation_logs RENAME COLUMN action TO moderation_status;

ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status TYPE INTEGER
USING CASE moderation_status
   WHEN 'pending' THEN 0
   WHEN 'approve' THEN 1
   WHEN 'reject' THEN 2
   WHEN 'delete' THEN 3
   WHEN 'flag' THEN 4
   ELSE NULL
END;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records_moderation_logs ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE records_moderation_logs ADD CONSTRAINT records_moderation_logs_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));

ALTER TABLE records_moderation_logs ADD new_status INTEGER NOT NULL DEFAULT 0 CHECK (new_status IN (0, 1, 2, 3, 4));
ALTER TABLE records_moderation_logs ADD previous_status INTEGER NOT NULL DEFAULT 0 CHECK (previous_status IN (0, 1, 2, 3, 4));
ALTER TABLE records_moderation_logs ADD modified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

ALTER INDEX idx_moderation_actions_record RENAME TO idx_records_moderation_logs_record;
ALTER INDEX idx_moderation_actions_admin RENAME TO idx_records_moderation_logs_admin;
ALTER SEQUENCE moderation_actions_id_seq RENAME TO records_moderation_logs_id_seq;

-- Edit table comment_moderation_actions
ALTER TABLE comment_moderation_actions RENAME TO comments_moderation_logs;
ALTER TABLE comments_moderation_logs DROP COLUMN reason;
ALTER TABLE comments_moderation_logs RENAME COLUMN admin_orcid TO reporter_orcid;
ALTER TABLE comments_moderation_logs RENAME COLUMN action TO previous_status;

ALTER TABLE comments_moderation_logs ALTER COLUMN previous_status TYPE INTEGER
USING CASE previous_status
   WHEN 'pending' THEN 0
   WHEN 'approve' THEN 1
   WHEN 'reject' THEN 2
   WHEN 'delete' THEN 3
   WHEN 'flag' THEN 4
   ELSE NULL
END;
ALTER TABLE comments_moderation_logs ALTER COLUMN previous_status SET NOT NULL;
ALTER TABLE comments_moderation_logs ALTER COLUMN previous_status SET DEFAULT 0;
ALTER TABLE comments_moderation_logs ADD CONSTRAINT comments_moderation_logs_previous_status_check CHECK (previous_status IN (0, 1, 2, 3, 4));

ALTER TABLE comments_moderation_logs ADD modified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
ALTER TABLE comments_moderation_logs ADD new_status INTEGER NOT NULL DEFAULT 0 CHECK (new_status IN (0, 1, 2, 3, 4));
ALTER INDEX idx_comment_moderation_actions_comment RENAME TO idx_comments_moderation_logs_comment;
ALTER INDEX idx_comment_moderation_actions_admin RENAME TO idx_comments_moderation_logs_reporter;
COMMENT ON COLUMN comments.moderation_status IS 'Moderation status: 0 = pending, 1 = approved, 2 = rejected, 3 = deleted, 4 = flagged';
ALTER SEQUENCE comment_moderation_actions_id_seq RENAME TO comments_moderation_logs_id_seq;

-- Edit table records
ALTER TABLE records ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records ALTER COLUMN moderation_status TYPE INTEGER
USING CASE moderation_status
   WHEN 'pending' THEN 0
   WHEN 'approved' THEN 1
   WHEN 'reject' THEN 2
   WHEN 'delete' THEN 3
   WHEN 'flag' THEN 4
   ELSE NULL
END;
ALTER TABLE records ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE records ADD CONSTRAINT records_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));

-- Edit table record_history
ALTER TABLE record_history RENAME TO records_logs;
ALTER TABLE records_logs ALTER COLUMN moderation_status DROP DEFAULT;
ALTER TABLE records_logs ALTER COLUMN moderation_status TYPE INTEGER
USING CASE moderation_status
   WHEN 'pending' THEN 0
   WHEN 'approved' THEN 1
   WHEN 'reject' THEN 2
   WHEN 'delete' THEN 3
   WHEN 'flag' THEN 4
   ELSE NULL
END;
ALTER TABLE records_logs ALTER COLUMN moderation_status SET NOT NULL;
ALTER TABLE records_logs ALTER COLUMN moderation_status SET DEFAULT 0;
ALTER TABLE records_logs ADD CONSTRAINT records_logs_moderation_status_check CHECK (moderation_status IN (0, 1, 2, 3, 4));
