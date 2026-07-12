package main

import (
	"context"
	"database/sql"
	"fmt"
	//"strings"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) error
	GetByRecordID(ctx context.Context, recordID string) ([]Comment, error)
	GetApprovedByRecordID(ctx context.Context, recordID string) ([]Comment, error)
	GetByID(ctx context.Context, id int64) (*Comment, error)
	GetPendingComments(ctx context.Context, limit, offset int) ([]Comment, int, error)
	ApproveComment(ctx context.Context, id int64) error
	RejectComment(ctx context.Context, id int64) error
	DeleteComment(ctx context.Context, id int64) error
	LogModerationAction(ctx context.Context, action CommentModerationAction) error
	GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationAction, error)
	GetCommentatorOrcid(ctx context.Context, id int64) (string, error)
	GetAllOrcids(ctx context.Context, recordId string) ([]string, error)
}

type PostgresCommentRepository struct {
	db *sql.DB
}

func NewPostgresCommentRepository(db *sql.DB) *PostgresCommentRepository {
	return &PostgresCommentRepository{db: db}
}

const commentErr = "Error: comment repository"

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
		return fmt.Errorf("%s: failed to create comment: %w", commentErr, err)
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
		return nil, fmt.Errorf("%s: failed to get comments by record id %q: %w", commentErr, recordID, err)
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
			return nil, fmt.Errorf("%s: failed to scan comment row: %w", commentErr, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: failed to read comment row: %w", commentErr, err)
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
		return nil, fmt.Errorf("%s: failed to get approved comments by record id %q: %w", commentErr, recordID, err)
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
			return nil, fmt.Errorf("%s: failed to scan approved comment row: %w", commentErr, err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: failed to read approved comment row: %w", commentErr, err)
	}

	return comments, nil
}

// GetByID retrieves a single comment by ID
func (r *PostgresCommentRepository) GetByID(ctx context.Context, id int64) (*Comment, error) {
	query := `
		SELECT id, record_id, commenter_name, commenter_orcid, content,
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE id = $1`

	var c Comment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.RecordID,
		&c.CommenterName,
		&c.CommenterOrcid,
		&c.Content,
		&c.ModerationStatus,
		&c.CreatedAt,
		&c.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// GetPendingComments retrieves comments awaiting moderation
func (r *PostgresCommentRepository) GetPendingComments(ctx context.Context, limit, offset int) ([]Comment, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM comments WHERE moderation_status = $1`,
		StatusPending).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT c.id, c.record_id, c.commenter_name, c.commenter_orcid, c.content,
		       c.moderation_status, c.created_at, c.modified_at
		FROM comments c
		WHERE c.moderation_status = $1
		ORDER BY c.created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, StatusPending, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		err := rows.Scan(
			&c.ID,
			&c.RecordID,
			&c.CommenterName,
			&c.CommenterOrcid,
			&c.Content,
			&c.ModerationStatus,
			&c.CreatedAt,
			&c.ModifiedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		comments = append(comments, c)
	}

	return comments, total, rows.Err()
}

// ApproveComment marks a comment as approved
func (r *PostgresCommentRepository) ApproveComment(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2`,
		StatusApproved, id)
	return err
}

// RejectComment marks a comment as rejected
func (r *PostgresCommentRepository) RejectComment(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE comments SET moderation_status = $1, modified_at = NOW() WHERE id = $2`,
		StatusRejected, id,
	)
	return err
}

// DeleteComment removes a comment
func (r *PostgresCommentRepository) DeleteComment(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, id)
	return err
}

// LogModerationAction records an admin action on a comment
func (r *PostgresCommentRepository) LogModerationAction(ctx context.Context, action CommentModerationAction) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO comment_moderation_actions (comment_id, admin_orcid, action, reason)
		 VALUES ($1, $2, $3, $4)`,
		action.CommentID,
		action.AdminOrcid,
		action.Action,
		action.Reason,
	)
	return err
}

// GetModerationHistory retrieves moderation history for a comment
func (r *PostgresCommentRepository) GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationAction, error) {
	query := `
		SELECT id, comment_id, admin_orcid, action, reason, created_at
		FROM comment_moderation_actions
		WHERE comment_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []CommentModerationAction
	for rows.Next() {
		var a CommentModerationAction
		var reason sql.NullString
		err := rows.Scan(
			&a.ID,
			&a.CommentID,
			&a.AdminOrcid,
			&a.Action,
			&reason,
			&a.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if reason.Valid {
			a.Reason = reason.String
		}
		actions = append(actions, a)
	}

	return actions, rows.Err()
}

func (r *PostgresCommentRepository) GetCommentatorOrcid(ctx context.Context, id int64) (string, error) {
	var commenterOrcid string
	err := r.db.QueryRowContext(ctx, `SELECT commenter_orcid FROM comments WHERE id = $1`, id).Scan(&commenterOrcid)
	if err != nil {
		return "", err
	}
	return commenterOrcid, nil
}

func (r *PostgresCommentRepository) GetAllOrcids(ctx context.Context, recordId string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT commenter_orcid FROM comments WHERE record_id = $1 AND commenter_orcid IS NOT NULL AND commenter_orcid != '' AND moderation_status = $2`, recordId, StatusApproved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var commentators []string
	for rows.Next() {
		var commentator string
		if err := rows.Scan(&commentator); err != nil {
			return commentators, err
		}
		commentators = append(commentators, commentator)
	}
	if err = rows.Err(); err != nil {
		return commentators, err
	}

	return commentators, nil
}
