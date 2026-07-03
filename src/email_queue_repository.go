package main

import (
	"context"
	"database/sql"
	"fmt"
)

type EmailQueueRepository interface {
	Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error)
	GetPending(ctx context.Context, limit int) ([]EmailQueue, error)
	MarkAsSent(ctx context.Context, id int64) error
	MarkAsFailed(ctx context.Context, id int64, errMsg string) error
	MarkForRetry(ctx context.Context, id int64, errMsg string) error
}

type PostgresEmailQueueRepository struct {
	db *sql.DB
}

func NewPostgresEmailQueueRepository(db *sql.DB) *PostgresEmailQueueRepository {
	return &PostgresEmailQueueRepository{db: db}
}

const repo = "Error: email queue repository"
const processingTimeout = "15 minutes"

// https://pkg.go.dev/database/sql#DB.QueryRowContext
// https://www.postgresql.org/docs/current/sql-insert.html
func (r *PostgresEmailQueueRepository) Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error) {
	var queue EmailQueue

	query := `INSERT INTO email_queue (record_id, comment_id, recipient_orcid, subject, body_text, body_html) VALUES($1, $2, $3, $4, $5, $6) RETURNING id, record_id, comment_id, recipient_orcid, subject, body_text, body_html, status, attempts, last_error, created_at, modified_at, sent_at`

	err := r.db.QueryRowContext(ctx, query, item.RecordID, item.CommentID, item.RecipientOrcid, item.Subject, item.BodyText, item.BodyHTML).Scan(&queue.Id, &queue.RecordID, &queue.CommentID, &queue.RecipientOrcid, &queue.Subject, &queue.BodyText, &queue.BodyHTML, &queue.Status, &queue.Attempts, &queue.LastError, &queue.CreatedAt, &queue.ModifiedAt, &queue.SentAt)

	if err != nil {
		return nil, fmt.Errorf("%s: failed to enqueue email for record %s: %w", repo, item.RecordID, err)
	}

	return &queue, nil
}

// https://www.postgresql.org/docs/current/sql-select.html
func (r *PostgresEmailQueueRepository) GetPending(ctx context.Context, limit int) ([]EmailQueue, error) {
	rows, err := r.db.QueryContext(ctx, `WITH processing AS (
        SELECT id FROM email_queue WHERE status = $1
        OR (status = $2 AND modified_at < NOW() - ($3::interval))
        ORDER BY created_at ASC
        LIMIT $4 FOR UPDATE SKIP LOCKED)
        UPDATE email_queue AS q
        SET status = $2, modified_at = NOW() FROM processing
        WHERE q.id = processing.id
        RETURNING q.id, q.record_id, q.comment_id, q.recipient_orcid, q.subject, q.body_text, q.body_html, q.status, q.attempts, q.last_error, q.created_at, q.modified_at, q.sent_at;`, PendingStatus, ProcessingStatus, processingTimeout, limit)

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get pending emails: %w", repo, err)
	}
	defer rows.Close()

	var pendingEmails []EmailQueue
	for rows.Next() {
		var email EmailQueue
		if err := rows.Scan(&email.Id, &email.RecordID, &email.CommentID, &email.RecipientOrcid, &email.Subject, &email.BodyText, &email.BodyHTML, &email.Status, &email.Attempts, &email.LastError, &email.CreatedAt, &email.ModifiedAt, &email.SentAt); err != nil {
			return nil, fmt.Errorf("%s: failed to scan pending email row: %w", repo, err)
		}
		pendingEmails = append(pendingEmails, email)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: failed to read pending email row: %w", repo, err)
	}

	return pendingEmails, nil
}

func (r *PostgresEmailQueueRepository) MarkAsSent(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE email_queue SET status = $1, last_error = NULL, sent_at = NOW() WHERE id = $2 AND status = $3`, SentStatus, id, ProcessingStatus)

	if err != nil {
		return fmt.Errorf("%s: failed to mark email queue item %d as sent: %w", repo, id, err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return fmt.Errorf("%s: expected to update 1 row for queue item %d, updated %d", repo, id, n)
	}
	return nil
}

func (r *PostgresEmailQueueRepository) MarkAsFailed(ctx context.Context, id int64, errMsg string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE email_queue SET status = $1, attempts = (attempts + 1), last_error = $2 WHERE id = $3 AND status = $4`, FailedStatus, errMsg, id, ProcessingStatus)

	if err != nil {
		return fmt.Errorf("%s: failed to mark email queue item %d as failed: %w", repo, id, err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return fmt.Errorf("%s: expected to update 1 row for queue item %d, updated %d", repo, id, n)
	}
	return nil
}

func (r *PostgresEmailQueueRepository) MarkForRetry(ctx context.Context, id int64, errMsg string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE email_queue SET status = $1, attempts = (attempts + 1), last_error = $2 WHERE id = $3 AND status = $4`, PendingStatus, errMsg, id, ProcessingStatus)

	if err != nil {
		return fmt.Errorf("%s: failed to mark email queue item %d as pending for retry: %w", repo, id, err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return fmt.Errorf("%s: expected to update 1 row for queue item %d, updated %d", repo, id, n)
	}
	return nil
}
