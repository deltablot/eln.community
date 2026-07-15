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
	GetByID(ctx context.Context, id int64) (*Comment, error)
	CountPending(ctx context.Context) (int, error)
	GetPending(ctx context.Context, limit int, offset int) ([]Comment, error)
	MarkAsApproved(ctx context.Context, id int64) error
	MarkAsRejected(ctx context.Context, id int64) error
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

const commentErr = "Error: comment repository: failed to"

func (r *PostgresCommentRepository) Create(ctx context.Context, comment *Comment) error {
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
		return fmt.Errorf("%s create comment: %w", commentErr, err)
	}
	return nil
}

func (r *PostgresCommentRepository) GetByRecordID(ctx context.Context, recordID string) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1
	    ORDER BY created_at ASC`, recordID)

	if err != nil {
		return nil, fmt.Errorf("%s get comments by record id %q: %w", commentErr, recordID, err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("%s scan comment row: %w", commentErr, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read comment rows: %w", commentErr, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) GetApprovedByRecordID(ctx context.Context, recordID string) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1 AND moderation_status = $2
	    ORDER BY created_at ASC`, recordID, StatusApproved)

	if err != nil {
		return nil, fmt.Errorf("%s get approved comments by record id %q: %w", commentErr, recordID, err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("%s scan approved comment row: %w", commentErr, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read approved comment rows: %w", commentErr, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) GetByID(ctx context.Context, id int64) (*Comment, error) {
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
		return nil, fmt.Errorf("%s get comment %d row: %w", commentErr, id, err)
	}

	return &comment, nil
}

func (r *PostgresCommentRepository) CountPending(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE moderation_status = $1`, StatusPending).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("%s count pending comments: %w", commentErr, err)
	}
	return total, nil
}

func (r *PostgresCommentRepository) GetPending(ctx context.Context, limit int, offset int) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.record_id, c.commenter_name, c.commenter_orcid, c.content, c.moderation_status, c.created_at, c.modified_at
		FROM comments c
		WHERE c.moderation_status = $1
		ORDER BY c.created_at ASC
		LIMIT $2 OFFSET $3`, StatusPending, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s get pending comments: %w", commentErr, err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("%s scan pending comment row: %w", commentErr, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read pending comment rows: %w", commentErr, err)
	}

	return comments, nil
}

func (r *PostgresCommentRepository) MarkAsApproved(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2 AND moderation_status != $3`, StatusApproved, id, StatusDeleted)
	if err != nil {
		return fmt.Errorf("%s mark comment %d as approved: %w", commentErr, id, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s get affected rows for comment %d approval: %w", commentErr, id, err)
	}
	if n != 1 {
		return fmt.Errorf("%s update: expected 1 row for comment %d, updated %d", commentErr, id, n)
	}
	return nil
}

func (r *PostgresCommentRepository) MarkAsRejected(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2 AND moderation_status != $3`, StatusRejected, id, StatusDeleted)
	if err != nil {
		return fmt.Errorf("%s mark comment %d as rejected: %w", commentErr, id, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s get affected rows for comment %d rejection: %w", commentErr, id, err)
	}
	if n != 1 {
		return fmt.Errorf("%s update: expected 1 row for comment %d, updated %d", commentErr, id, n)
	}
	return nil
}

// Hard delete: this permanently removes the comment.
// TODO: consider reserving hard deletes for maintenance tasks and using StatusDeleted for soft deletes in the moderation workflow.
func (r *PostgresCommentRepository) DeleteComment(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%s delete comment %d: %w", commentErr, id, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s get affected rows for comment %d deletion: %w", commentErr, id, err)
	}
	if n != 1 {
		return fmt.Errorf("%s delete: expected 1 row for comment %d, deleted %d", commentErr, id, n)
	}
	return nil
}

func (r *PostgresCommentRepository) AuthorDeleteComment(ctx context.Context, id int64, commentatorOrcid string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1 AND commenter_orcid = $2`, id, commentatorOrcid)
	if err != nil {
		return fmt.Errorf("%s delete comment %d: %w", commentErr, id, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s get affected rows for comment %d deletion: %w", commentErr, id, err)
	}
	if n != 1 {
		return fmt.Errorf("%s delete: expected 1 row for comment %d, deleted %d", commentErr, id, n)
	}
	return nil
}

func (r *PostgresCommentRepository) CreateModerationHistory(ctx context.Context, moderation CommentModerationHistory) error {
	query := `INSERT INTO comment_moderation_history (comment_id, admin_orcid, previous_status, new_status, reason)
		 VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, query,
		moderation.CommentID,
		moderation.AdminOrcid,
		moderation.PreviousStatus,
		moderation.NewStatus,
		moderation.Reason,
	)
	if err != nil {
		return fmt.Errorf("%s create log for comment: %w", commentErr, err)
	}
	return nil
}

func (r *PostgresCommentRepository) GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationHistory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, comment_id, admin_orcid, previous_status, new_status, reason, created_at, modified_at
		FROM comment_moderation_history
		WHERE comment_id = $1
		ORDER BY created_at DESC`, commentID)

	if err != nil {
		return nil, fmt.Errorf("%s get history moderation comment rows: %w", commentErr, err)
	}
	defer rows.Close()

	var moderations []CommentModerationHistory
	for rows.Next() {
		var moderation CommentModerationHistory
		if err := rows.Scan(
			&moderation.ID,
			&moderation.CommentID,
			&moderation.AdminOrcid,
			&moderation.PreviousStatus,
			&moderation.NewStatus,
			&moderation.Reason,
			&moderation.CreatedAt,
			&moderation.ModifiedAt,
		); err != nil {
			return nil, fmt.Errorf("%s scan history moderation comment row: %w", commentErr, err)
		}
		moderations = append(moderations, moderation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read history moderation comment rows: %w", commentErr, err)
	}

	return moderations, rows.Err()
}

func (r *PostgresCommentRepository) GetAllOrcids(ctx context.Context, recordId string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT commenter_orcid FROM comments WHERE record_id = $1 AND commenter_orcid IS NOT NULL AND commenter_orcid != '' AND moderation_status = $2`, recordId, StatusApproved)
	if err != nil {
		return nil, fmt.Errorf("%s get all orcids for record id %d: %w", commentErr, recordId, err)
	}
	defer rows.Close()

	var commentators []string
	for rows.Next() {
		var commentator string
		if err := rows.Scan(&commentator); err != nil {
			return nil, fmt.Errorf("%s scan orcid row for record %d: %w", commentErr, recordId, err)
		}
		commentators = append(commentators, commentator)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s read orcid rows for record %d: %w", commentErr, recordId, err)
	}
	return commentators, nil
}
