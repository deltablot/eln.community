package main

import (
	"context"
	"database/sql"
)

type EmailQueueRepository interface {
	Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error)
	GetPendingEmails(ctx context.Context, limit int) ([]EmailQueue, error)
    MarkEmailAsSent(ctx context.Context, id int64) error
    MarkEmailAsFailed(ctx context.Context, id int64, errMsg string) error
}

type PostgresEmailQueueRepository struct {
	db *sql.DB
}

func NewPostgresEmailQueueRepository(db *sql.DB) *PostgresEmailQueueRepository {
	return &PostgresEmailQueueRepository{db: db}
}

func (r *PostgresEmailQueueRepository) Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error) {
	var queue EmailQueue

	query := `INSERT INTO email_queue (record_id, comment_id, recipient_orcid, send_from, subject, body, recipient_type, notification_type) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, record_id, comment_id, recipient_orcid, send_from, subject, body, recipient_type, notification_type, status, attempts, last_error, created_at, sent_at`

	err := r.db.QueryRowContext(ctx, query, item.RecordID, item.CommentID, item.RecipientOrcid, item.SendFrom, item.Subject, item.Body, item.RecipientType, item.NotificationType).Scan(&queue.Id, &queue.RecordID, &queue.CommentID, &queue.RecipientOrcid, &queue.SendFrom, &queue.Subject, &queue.Body, &queue.RecipientType, &queue.NotificationType, &queue.Status, &queue.Attempts, &queue.LastError, &queue.CreatedAt, &queue.SentAt)

	if err != nil {
		return nil, err
	}

	return &queue, nil
}

func (r *PostgresEmailQueueRepository) GetPendingEmails(ctx context.Context, limit int) ([]EmailQueue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, record_id, comment_id, recipient_orcid, send_from, subject, body, recipient_type, notification_type, status, attempts, last_error, created_at, sent_at FROM email_queue WHERE status = 'pending' ORDER BY created_at ASC  LIMIT $1`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pendingEmails []EmailQueue
	for rows.Next() {
		var email EmailQueue
		if err := rows.Scan(&email.Id, &email.RecordID, &email.CommentID, &email.RecipientOrcid, &email.SendFrom, &email.Subject, &email.Body, &email.RecipientType, &email.NotificationType, &email.Status, &email.Attempts, &email.LastError, &email.CreatedAt, &email.SentAt); err != nil {
			return nil, err
		}
		pendingEmails = append(pendingEmails, email)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pendingEmails, nil
}

func (r *PostgresEmailQueueRepository) MarkEmailAsSent(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE email_queue SET status = 'sent', last_error = NULL, sent_at = NOW() WHERE id = $1`, id)

	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresEmailQueueRepository) MarkEmailAsFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE email_queue SET status = 'failed', attempts = (attempts + 1), last_error = $1 WHERE id = $2`, errMsg, id)

	if err != nil {
		return err
	}
	return nil
}
