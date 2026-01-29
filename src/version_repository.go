package main

import (
	"context"
	"database/sql"
)

// HistoryRepository defines the interface for record history operations
type HistoryRepository interface {
	GetHistory(ctx context.Context, recordID string) ([]RecordHistory, error)
	GetVersion(ctx context.Context, recordID string, version int) (*RecordHistory, error)
}

// PostgresHistoryRepository implements HistoryRepository using PostgreSQL
type PostgresHistoryRepository struct {
	db *sql.DB
}

// NewPostgresHistoryRepository creates a new PostgreSQL history repository
func NewPostgresHistoryRepository(db *sql.DB) *PostgresHistoryRepository {
	return &PostgresHistoryRepository{db: db}
}

// GetHistory retrieves all historical versions of a record
func (r *PostgresHistoryRepository) GetHistory(ctx context.Context, recordID string) ([]RecordHistory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT history_id, record_id, version, s3_key, name, sha256, metadata,
		       uploader_name, uploader_orcid, download_count,
		       created_at, modified_at, archived_at, change_type, license
		FROM record_history
		WHERE record_id = $1
		ORDER BY version DESC
	`, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []RecordHistory
	for rows.Next() {
		var h RecordHistory
		if err := rows.Scan(
			&h.HistoryId, &h.RecordId, &h.Version, &h.S3Key, &h.Name, &h.Sha256, &h.Metadata,
			&h.UploaderName, &h.UploaderOrcid, &h.DownloadCount,
			&h.CreatedAt, &h.ModifiedAt, &h.ArchivedAt, &h.ChangeType, &h.License,
		); err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

// GetVersion retrieves a specific version from history
func (r *PostgresHistoryRepository) GetVersion(ctx context.Context, recordID string, version int) (*RecordHistory, error) {
	var h RecordHistory
	err := r.db.QueryRowContext(ctx, `
		SELECT history_id, record_id, version, s3_key, name, sha256, metadata,
		       uploader_name, uploader_orcid, download_count,
		       created_at, modified_at, archived_at, change_type, license
		FROM record_history
		WHERE record_id = $1 AND version = $2
	`, recordID, version).Scan(
		&h.HistoryId, &h.RecordId, &h.Version, &h.S3Key, &h.Name, &h.Sha256, &h.Metadata,
		&h.UploaderName, &h.UploaderOrcid, &h.DownloadCount,
		&h.CreatedAt, &h.ModifiedAt, &h.ArchivedAt, &h.ChangeType, &h.License,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return &h, nil
}
