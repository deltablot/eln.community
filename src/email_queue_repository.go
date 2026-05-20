package main

import (
	"context"
	"database/sql"
)

type EmailQueueRepository interface {
	Enqueue(ctx context.Context, item *EmailQueueItem) (*EmailQueueItem, error)
}

type PostgresEmailQueueRepository struct {
	db *sql.DB
}

func NewPostgresEmailQueueRepository(db *sql.DB) *PostgresEmailQueueRepository {
	return &PostgresEmailQueueRepository{db: db}
}

func (r *PostgresEmailQueueRepository) Enqueue(ctx context.Context, item *EmailQueueItem) (*EmailQueueItem, error) {
	var queue EmailQueueItem

	query := `INSERT INTO email_queue (record_id, comment_id, recipient_orcid, send_from, subject, body, recipient_type, notification_type) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, record_id, comment_id, recipient_orcid, send_from, subject, body, recipient_type, notification_type, status, attempts, last_error, created_at, sent_at`
	err := r.db.QueryRowContext(ctx, query, item.RecordID, item.CommentID, item.RecipientOrcid, item.SendFrom, item.Subject, item.Body, item.RecipientType, item.NotificationType).Scan(&queue.Id, &queue.RecordID, &queue.CommentID, &queue.RecipientOrcid, &queue.SendFrom, &queue.Subject, &queue.Body, &queue.RecipientType, &queue.NotificationType, &queue.Status, &queue.Attempts, &queue.LastError, &queue.CreatedAt, &queue.SentAt)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}
