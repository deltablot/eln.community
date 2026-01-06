package main

import (
	"context"
	"database/sql"
	"os"
	"time"
)

// ModerationStatus represents the review state of a record
type ModerationStatus string

const (
	StatusPendingReview ModerationStatus = "pending_review"
	StatusApproved      ModerationStatus = "approved"
	StatusRejected      ModerationStatus = "rejected"
	StatusFlagged       ModerationStatus = "flagged"
)

// ModerationAction represents an admin action on a record
type ModerationAction struct {
	ID         int64
	RecordID   string
	AdminOrcid string
	Action     string // "approve", "reject", "flag"
	Reason     string
	CreatedAt  time.Time
}

// ModerationRepository handles moderation data access
type ModerationRepository interface {
	GetRecordStatus(ctx context.Context, recordID string) (ModerationStatus, error)
	SetRecordStatus(ctx context.Context, recordID string, status ModerationStatus) error
	GetPendingRecords(ctx context.Context, limit, offset int) ([]Record, int, error)
	GetFlaggedRecords(ctx context.Context, limit, offset int) ([]Record, int, error)
	LogModerationAction(ctx context.Context, action ModerationAction) error
	GetModerationHistory(ctx context.Context, recordID string) ([]ModerationAction, error)
}

// PostgresModerationRepository implements ModerationRepository
type PostgresModerationRepository struct {
	db           *sql.DB
	categoryRepo CategoryRepository
	rorRepo      RorRepository
}

func NewPostgresModerationRepository(db *sql.DB, categoryRepo CategoryRepository, rorRepo RorRepository) *PostgresModerationRepository {
	return &PostgresModerationRepository{
		db:           db,
		categoryRepo: categoryRepo,
		rorRepo:      rorRepo,
	}
}

func (r *PostgresModerationRepository) GetRecordStatus(ctx context.Context, recordID string) (ModerationStatus, error) {
	var status string
	err := r.db.QueryRowContext(ctx,
		"SELECT moderation_status FROM records WHERE id = $1",
		recordID,
	).Scan(&status)
	if err != nil {
		return "", err
	}
	return ModerationStatus(status), nil
}

func (r *PostgresModerationRepository) SetRecordStatus(ctx context.Context, recordID string, status ModerationStatus) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE records SET moderation_status = $1, modified_at = NOW() WHERE id = $2",
		string(status), recordID,
	)
	return err
}

func (r *PostgresModerationRepository) GetPendingRecords(ctx context.Context, limit, offset int) ([]Record, int, error) {
	return r.getRecordsByStatus(ctx, StatusPendingReview, limit, offset)
}

func (r *PostgresModerationRepository) GetFlaggedRecords(ctx context.Context, limit, offset int) ([]Record, int, error) {
	return r.getRecordsByStatus(ctx, StatusFlagged, limit, offset)
}

func (r *PostgresModerationRepository) getRecordsByStatus(ctx context.Context, status ModerationStatus, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM records WHERE moderation_status = $1",
		string(status),
	).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get records
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, sha256, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count
		 FROM records
		 WHERE moderation_status = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		string(status), limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		err := rows.Scan(
			&rec.Id,
			&rec.Name,
			&rec.Sha256,
			&rec.Metadata,
			&rec.CreatedAt,
			&rec.ModifiedAt,
			&rec.UploaderName,
			&rec.UploaderOrcid,
			&rec.DownloadCount,
		)
		if err != nil {
			return nil, 0, err
		}

		// Load categories
		categories, err := r.categoryRepo.GetRecordCategories(ctx, rec.Id)
		if err == nil {
			rec.Categories = categories
		}

		// Load ROR IDs
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, rec.Id)
		if err == nil {
			rec.RorIds = rorIds
		}

		records = append(records, rec)
	}

	return records, totalCount, rows.Err()
}

func (r *PostgresModerationRepository) LogModerationAction(ctx context.Context, action ModerationAction) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO moderation_actions (record_id, admin_orcid, action, reason, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		action.RecordID, action.AdminOrcid, action.Action, action.Reason, time.Now(),
	)
	return err
}

func (r *PostgresModerationRepository) GetModerationHistory(ctx context.Context, recordID string) ([]ModerationAction, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, record_id, admin_orcid, action, reason, created_at
		 FROM moderation_actions
		 WHERE record_id = $1
		 ORDER BY created_at DESC`,
		recordID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ModerationAction
	for rows.Next() {
		var action ModerationAction
		err := rows.Scan(
			&action.ID,
			&action.RecordID,
			&action.AdminOrcid,
			&action.Action,
			&action.Reason,
			&action.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	return actions, rows.Err()
}

// IsModerationEnabled checks if moderation is enabled via environment variable
// Default is true (enabled)
func IsModerationEnabled() bool {
	enabled := os.Getenv("MODERATION_ENABLED")
	// Default to enabled if not set or set to anything other than "false"
	return enabled != "false"
}

// GetInitialModerationStatus returns the initial status for new records
// based on whether moderation is enabled
func GetInitialModerationStatus() ModerationStatus {
	if IsModerationEnabled() {
		return StatusPendingReview
	}
	return StatusApproved
}
