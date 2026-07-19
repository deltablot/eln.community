package main

import (
	"context"
	"database/sql"
	"fmt"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) error
	GetByRecordID(ctx context.Context, recordID string) ([]Comment, error)
	GetApprovedByRecordID(ctx context.Context, recordID string) ([]Comment, error)
	GetVisibleByRecordID(ctx context.Context, recordID string, commenterOrcid string) ([]Comment, error)
	GetByID(ctx context.Context, id int64) (*Comment, error)
	CountPending(ctx context.Context) (int, error)
	GetPending(ctx context.Context, limit int, offset int) ([]Comment, error)
	MarkAsApproved(ctx context.Context, id int64) error
	MarkAsRejected(ctx context.Context, id int64) error
	MarkAsFlagged(ctx context.Context, id int64) error
	DeleteComment(ctx context.Context, id int64) error
	AuthorDeleteComment(ctx context.Context, id int64, commentatorOrcid string) error
	CreateModerationHistory(ctx context.Context, action CommentModerationHistory) error
	GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationHistory, error)
	GetAllOrcids(ctx context.Context, recordId string) ([]string, error)
}

type PostgresCommentRepository struct {
	db *sql.DB
}

func NewPostgresCommentRepository(db *sql.DB) *PostgresCommentRepository {
	return &PostgresCommentRepository{db: db}
}

const commentErr = "comment repository: failed to"

func scanAllComments(rows *sql.Rows, fn string) ([]Comment, error) {
	source := errorSource(fn, commentErr)
	var comments []Comment
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(
			&comment.ID,
			&comment.RecordID,
			&comment.CommenterName,
			&comment.CommenterOrcid,
			&comment.Content,
			&comment.ModerationStatus,
			&comment.CreatedAt,
			&comment.ModifiedAt,
		); err != nil {
			return nil, fmt.Errorf("%s scan comment row: %w", source, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *PostgresCommentRepository) Create(ctx context.Context, comment *Comment) error {
	source := errorSource("Create", commentErr)
	query := `
      INSERT INTO comments (record_id, commenter_name, commenter_orcid, content, moderation_status)
      VALUES ($1, $2, $3, $4, $5)
      RETURNING id, created_at, modified_at`
	err := r.db.QueryRowContext(ctx, query,
		comment.RecordID,
		comment.CommenterName,
		comment.CommenterOrcid,
		comment.Content,
		comment.ModerationStatus,
	).Scan(&comment.ID, &comment.CreatedAt, &comment.ModifiedAt)
	if err != nil {
		return fmt.Errorf("%s create comment: %w", source, err)
	}

	return nil
}

func (r *PostgresCommentRepository) GetByRecordID(ctx context.Context, recordID string) ([]Comment, error) {
	source := errorSource("GetByRecordID", commentErr)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1
	    ORDER BY created_at ASC`, recordID)
	if err != nil {
		return nil, fmt.Errorf("%s get comments by record id %q: %w", source, recordID, err)
	}
	defer rows.Close()

	comments, err := scanAllComments(rows, "GetByRecordID")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read comment rows: %w", source, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) GetApprovedByRecordID(ctx context.Context, recordID string) ([]Comment, error) {
	source := errorSource("GetApprovedByRecordID", commentErr)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1 AND moderation_status = $2 AND moderation_status != $3
	    ORDER BY created_at ASC`, recordID, StatusApproved, StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("%s get approved comments by record id %q: %w", source, recordID, err)
	}
	defer rows.Close()
	comments, err := scanAllComments(rows, "GetApprovedByRecordID")

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read approved comment rows: %w", source, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) GetVisibleByRecordID(ctx context.Context, recordID string, commenterOrcid string) ([]Comment, error) {
	source := errorSource("GetVisibleByRecordID", commentErr)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1 AND (moderation_status = $2 OR (commenter_orcid = $3 AND moderation_status != $4))
	    ORDER BY created_at ASC`, recordID, StatusApproved, commenterOrcid, StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("%s get approved comments by record id %q: %w", source, recordID, err)
	}
	defer rows.Close()
	comments, err := scanAllComments(rows, "GetApprovedByRecordID")

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read approved comment rows: %w", source, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) GetByID(ctx context.Context, id int64) (*Comment, error) {
	source := errorSource("GetByID", commentErr)
	query := `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE id = $1`
	var comment Comment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&comment.ID,
		&comment.RecordID,
		&comment.CommenterName,
		&comment.CommenterOrcid,
		&comment.Content,
		&comment.ModerationStatus,
		&comment.CreatedAt,
		&comment.ModifiedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%s get comment %d row: %w", source, id, err)
	}

	return &comment, nil
}

func (r *PostgresCommentRepository) CountPending(ctx context.Context) (int, error) {
	source := errorSource("CountPending", commentErr)
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE moderation_status = $1`, StatusPending).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("%s count pending comments: %w", source, err)
	}

	return total, nil
}

func (r *PostgresCommentRepository) GetPending(ctx context.Context, limit int, offset int) ([]Comment, error) {
	source := errorSource("GetPending", commentErr)
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.record_id, c.commenter_name, c.commenter_orcid, c.content, c.moderation_status, c.created_at, c.modified_at
		FROM comments c
		WHERE c.moderation_status = $1
		ORDER BY c.created_at ASC
		LIMIT $2 OFFSET $3`, StatusPending, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s get pending comments: %w", source, err)
	}
	defer rows.Close()

	comments, err := scanAllComments(rows, "GetPending")

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read pending comment rows: %w", source, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) MarkAsApproved(ctx context.Context, id int64) error {
	return r.setModerationIfNotDeleted(ctx, id, StatusApproved)
}

func (r *PostgresCommentRepository) MarkAsRejected(ctx context.Context, id int64) error {
	return r.setModerationIfNotDeleted(ctx, id, StatusRejected)
}

func (r *PostgresCommentRepository) setModerationIfNotDeleted(ctx context.Context, id int64, status ModerationStatus) error {
	source := errorSource("SetModerationIfNotDeleted", commentErr)
	res, err := r.db.ExecContext(ctx, `UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2 AND moderation_status != $3`, status, id, StatusDeleted)
	if err != nil {
		return fmt.Errorf("%s mark comment %d as approved: %w", source, id, err)
	}
	n, err := res.RowsAffected()
	errorUpdateRow(source, "comment", id, err, n)

	return nil
}

func (r *PostgresCommentRepository) MarkAsFlagged(ctx context.Context, id int64) error {
	source := errorSource("MarkAsFlagged", commentErr)
	res, err := r.db.ExecContext(ctx, `UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2 AND moderation_status = $3`, StatusFlagged, id, StatusApproved)
	if err != nil {
		return fmt.Errorf("%s mark comment %d as flagged: %w", source, id, err)
	}
	n, err := res.RowsAffected()
	errorUpdateRow(source, "comment", id, err, n)

	return nil
}

func (r *PostgresCommentRepository) DeleteComment(ctx context.Context, id int64) error {
	source := errorSource("DeleteComment", commentErr)
	res, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%s delete comment %d: %w", source, id, err)
	}
	n, err := res.RowsAffected()
	errorUpdateRow(source, "comment", id, err, n)

	return nil
}

func (r *PostgresCommentRepository) AuthorDeleteComment(ctx context.Context, id int64, commentatorOrcid string) error {
	source := errorSource("AuthorDeleteComment", commentErr)
	res, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1 AND commenter_orcid = $2`, id, commentatorOrcid)
	if err != nil {
		return fmt.Errorf("%s delete comment %d: %w", source, id, err)
	}
	n, err := res.RowsAffected()
	errorUpdateRow(source, "comment", id, err, n)

	return nil
}

func (r *PostgresCommentRepository) CreateModerationHistory(ctx context.Context, moderation CommentModerationHistory) error {
	source := errorSource("CreateModerationHistory", commentErr)
	query := `INSERT INTO comment_moderation_history (comment_id, reporter_orcid, previous_status, new_status)
		 VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query,
		moderation.CommentID,
		moderation.ReporterOrcid,
		moderation.PreviousStatus,
		moderation.NewStatus,
	)
	if err != nil {
		return fmt.Errorf("%s create log for comment: %w", source, err)
	}

	return nil
}

func (r *PostgresCommentRepository) GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationHistory, error) {
	source := errorSource("GetModerationHistory", commentErr)
	rows, err := r.db.QueryContext(ctx, `SELECT id, comment_id, reporter_orcid, previous_status, new_status, created_at, modified_at
		FROM comment_moderation_history
		WHERE comment_id = $1
		ORDER BY created_at DESC`, commentID)
	if err != nil {
		return nil, fmt.Errorf("%s get history moderation comment rows: %w", source, err)
	}
	defer rows.Close()
	var moderations []CommentModerationHistory
	for rows.Next() {
		var moderation CommentModerationHistory
		if err := rows.Scan(
			&moderation.ID,
			&moderation.CommentID,
			&moderation.ReporterOrcid,
			&moderation.PreviousStatus,
			&moderation.NewStatus,
			&moderation.CreatedAt,
			&moderation.ModifiedAt,
		); err != nil {
			return nil, fmt.Errorf("%s scan history moderation comment row: %w", source, err)
		}
		moderations = append(moderations, moderation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read history moderation comment rows: %w", source, err)
	}

	return moderations, rows.Err()
}

func (r *PostgresCommentRepository) GetAllOrcids(ctx context.Context, recordId string) ([]string, error) {
	source := errorSource("GetAllOrcids", commentErr)
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT commenter_orcid FROM comments WHERE record_id = $1 AND commenter_orcid IS NOT NULL AND commenter_orcid != '' AND moderation_status = $2`, recordId, StatusApproved)
	if err != nil {
		return nil, fmt.Errorf("%s get all orcids for record id %d: %w", source, recordId, err)
	}
	defer rows.Close()
	var commentators []string
	for rows.Next() {
		var commentator string
		if err := rows.Scan(&commentator); err != nil {
			return nil, fmt.Errorf("%s scan orcid row for record %d: %w", source, recordId, err)
		}
		commentators = append(commentators, commentator)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read orcid rows for record %d: %w", source, recordId, err)
	}

	return commentators, nil
}
