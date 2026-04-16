package app

import (
	"context"
	"database/sql"
	"html"
	"strings"
	"time"
)

// Comment represents a user comment on a record
type Comment struct {
	ID               int64            `json:"id"`
	RecordID         string           `json:"record_id"`
	CommenterName    string           `json:"commenter_name"`
	CommenterOrcid   string           `json:"commenter_orcid"`
	Content          string           `json:"content"`
	ModerationStatus ModerationStatus `json:"moderation_status"`
	CreatedAt        time.Time        `json:"created_at"`
	ModifiedAt       time.Time        `json:"modified_at"`
}

// CommentModerationAction represents an admin action on a comment
type CommentModerationAction struct {
	ID         int64
	CommentID  int64
	AdminOrcid string
	Action     string // "approve", "reject", "delete"
	Reason     string
	CreatedAt  time.Time
}

// CommentRepository defines the interface for comment data operations
type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) error
	GetByRecordID(ctx context.Context, recordID string, includeModerated bool) ([]Comment, error)
	GetByID(ctx context.Context, id int64) (*Comment, error)
	GetPendingComments(ctx context.Context, limit, offset int) ([]Comment, int, error)
	ApproveComment(ctx context.Context, id int64) error
	RejectComment(ctx context.Context, id int64) error
	DeleteComment(ctx context.Context, id int64) error
	LogModerationAction(ctx context.Context, action CommentModerationAction) error
	GetModerationHistory(ctx context.Context, commentID int64) ([]CommentModerationAction, error)
}

// PostgresCommentRepository implements CommentRepository
type PostgresCommentRepository struct {
	db *sql.DB
}

func NewPostgresCommentRepository(db *sql.DB) *PostgresCommentRepository {
	return &PostgresCommentRepository{db: db}
}

// Create adds a new comment (sanitized as plain text)
func (r *PostgresCommentRepository) Create(ctx context.Context, comment *Comment) error {
	// Sanitize content: strip any HTML and trim whitespace
	comment.Content = sanitizeCommentContent(comment.Content)

	if len(strings.TrimSpace(comment.Content)) == 0 {
		return ErrEmptyComment
	}

	query := `
		INSERT INTO comments (record_id, commenter_name, commenter_orcid, content, moderation_status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, modified_at`

	return r.db.QueryRowContext(ctx, query,
		comment.RecordID,
		comment.CommenterName,
		comment.CommenterOrcid,
		comment.Content,
		comment.ModerationStatus,
	).Scan(&comment.ID, &comment.CreatedAt, &comment.ModifiedAt)
}

// GetByRecordID retrieves comments for a specific record
func (r *PostgresCommentRepository) GetByRecordID(ctx context.Context, recordID string, includeModerated bool) ([]Comment, error) {
	query := `
		SELECT id, record_id, commenter_name, commenter_orcid, content, 
		       moderation_status, created_at, modified_at
		FROM comments
		WHERE record_id = $1`

	if !includeModerated {
		query += ` AND moderation_status = 'approved'`
	}

	query += ` ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, recordID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, rows.Err()
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
		`SELECT COUNT(*) FROM comments WHERE moderation_status = 'pending_review'`,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT c.id, c.record_id, c.commenter_name, c.commenter_orcid, c.content,
		       c.moderation_status, c.created_at, c.modified_at
		FROM comments c
		WHERE c.moderation_status = 'pending_review'
		ORDER BY c.created_at ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
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
		`UPDATE comments SET moderation_status = 'approved', modified_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// RejectComment marks a comment as rejected
func (r *PostgresCommentRepository) RejectComment(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE comments SET moderation_status = 'rejected', modified_at = NOW() WHERE id = $1`,
		id,
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

// sanitizeCommentContent strips HTML and returns plain text
func sanitizeCommentContent(content string) string {
	// HTML escape to prevent any HTML rendering
	content = html.EscapeString(content)
	// Trim whitespace
	content = strings.TrimSpace(content)
	return content
}

var ErrEmptyComment = sql.ErrNoRows
