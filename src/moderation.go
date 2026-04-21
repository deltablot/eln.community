package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	ID          int64
	RecordID    string
	AdminOrcid  string
	Action      string // "approve", "reject", "flag"
	Reason      string
	VersionName string // Name of the version that was moderated
	CreatedAt   time.Time
}

// PendingItem represents an item in the moderation queue
type PendingItem struct {
	RecordID       string          `json:"record_id"`
    Name           string          `json:"name"`
	Description    string          `json:"description"`
	Sha256         string          `json:"sha256"`
	Metadata       json.RawMessage `json:"metadata"`
	MetadataPretty string          `json:"-"`
	CreatedAt      time.Time       `json:"created_at"`
	ModifiedAt     time.Time       `json:"modified_at"`
	UploaderName   string          `json:"uploader_name"`
	UploaderOrcid  string          `json:"uploader_orcid"`
	Categories     []Category      `json:"categories,omitempty"`
	RorIds         []string        `json:"rors,omitempty"`
	IsNewEntry     bool            `json:"is_new_entry"`              // true if new entry, false if pending version
	Version        int             `json:"version,omitempty"`         // version number if pending version
	CurrentVersion string          `json:"current_version,omitempty"` // current approved version info if pending version
}

// ModerationRepository handles moderation data access
type ModerationRepository interface {
	GetRecordStatus(ctx context.Context, recordID string) (ModerationStatus, error)
	SetRecordStatus(ctx context.Context, recordID string, status ModerationStatus) error
	ApprovePendingVersion(ctx context.Context, recordID string) error
	RejectPendingVersion(ctx context.Context, recordID string) error
	GetPendingRecords(ctx context.Context, limit, offset int) ([]Record, int, error)
	GetPendingItems(ctx context.Context, limit, offset int) ([]PendingItem, int, error)
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

// ApprovePendingVersion moves a pending version from history to the main record
func (r *PostgresModerationRepository) ApprovePendingVersion(ctx context.Context, recordID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the pending version from history
	var historyID int64
	var s3Key, name, sha256, description string
	var metadata []byte
	err = tx.QueryRowContext(ctx,
		`SELECT history_id, s3_key, name, sha256, metadata, description
		 FROM record_history
		 WHERE record_id = $1 AND moderation_status = 'pending' AND change_type = 'PENDING_VERSION'
		 ORDER BY version DESC
		 LIMIT 1`,
		recordID,
	).Scan(&historyID, &s3Key, &name, &sha256, &metadata, &description)
	if err == sql.ErrNoRows {
		// No pending version, just approve the main record (status change only, no history entry)
		return r.SetRecordStatus(ctx, recordID, StatusApproved)
	}
	if err != nil {
		return err
	}

	// Set a session variable to tell the trigger to skip (avoid duplicate history entries)
	// The pending version is already in history, so we don't need the trigger to archive it again
	_, err = tx.ExecContext(ctx, `SET LOCAL app.skip_audit_trigger = 'true'`)
	if err != nil {
		return err
	}

	// Update the main record with the pending version data
	_, err = tx.ExecContext(ctx,
		`UPDATE records
		 SET s3_key = $2, name = $3, sha256 = $4, metadata = $5, description = $6, moderation_status = 'approved', modified_at = NOW()
		 WHERE id = $1`,
		recordID, s3Key, name, sha256, metadata, description,
	)
	if err != nil {
		return err
	}

	// Mark the pending version as approved in history
	_, err = tx.ExecContext(ctx,
		`UPDATE record_history
		 SET moderation_status = 'approved', change_type = 'UPDATE'
		 WHERE history_id = $1`,
		historyID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RejectPendingVersion marks the latest pending version as rejected
func (r *PostgresModerationRepository) RejectPendingVersion(ctx context.Context, recordID string) error {
	// Update only the latest pending version in history to rejected
	result, err := r.db.ExecContext(ctx,
		`UPDATE record_history
		 SET moderation_status = 'rejected'
		 WHERE history_id = (
			SELECT history_id
			FROM record_history
			WHERE record_id = $1
			AND moderation_status = 'pending'
			AND change_type = 'PENDING_VERSION'
			ORDER BY version DESC
			LIMIT 1
		 )`,
		recordID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no pending version was found, reject the main record
	if rowsAffected == 0 {
		return r.SetRecordStatus(ctx, recordID, StatusRejected)
	}

	return nil
}

func (r *PostgresModerationRepository) GetPendingRecords(ctx context.Context, limit, offset int) ([]Record, int, error) {
	// Get pending records from main table AND pending versions from history
	// First, get count
	var totalCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT id FROM records WHERE moderation_status = $1
			UNION
			SELECT DISTINCT record_id as id FROM record_history WHERE moderation_status = 'pending' AND change_type = 'PENDING_VERSION'
		) AS pending_items`,
		string(StatusPendingReview),
	).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get records - combine main table pending records and records with pending versions
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT r.id, r.name, r.description, r.sha256, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count, r.moderation_status
		 FROM records r
		 LEFT JOIN record_history rh ON r.id = rh.record_id AND rh.moderation_status = 'pending' AND rh.change_type = 'PENDING_VERSION'
		 WHERE r.moderation_status = $1 OR rh.record_id IS NOT NULL
		 ORDER BY r.created_at DESC
		 LIMIT $2 OFFSET $3`,
		string(StatusPendingReview), limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var moderationStatus string
		err := rows.Scan(
			&rec.Id,
			&rec.Name,
			&rec.Description,
			&rec.Sha256,
			&rec.Metadata,
			&rec.CreatedAt,
			&rec.ModifiedAt,
			&rec.UploaderName,
			&rec.UploaderOrcid,
			&rec.DownloadCount,
			&moderationStatus,
		)
		if err != nil {
			return nil, 0, err
		}
		rec.ModerationStatus = ModerationStatus(moderationStatus)

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

// GetPendingItems retrieves all pending items (new entries and pending versions) for moderation
func (r *PostgresModerationRepository) GetPendingItems(ctx context.Context, limit, offset int) ([]PendingItem, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT id FROM records WHERE moderation_status = $1
			UNION ALL
			SELECT DISTINCT record_id as id
			FROM record_history rh
			WHERE rh.moderation_status = 'pending'
			AND rh.change_type = 'PENDING_VERSION'
			AND rh.version = (
				SELECT MAX(version)
				FROM record_history rh2
				WHERE rh2.record_id = rh.record_id
				AND rh2.moderation_status = 'pending'
				AND rh2.change_type = 'PENDING_VERSION'
			)
		) AS pending_items`,
		string(StatusPendingReview),
	).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get pending items - both new entries and pending versions
	rows, err := r.db.QueryContext(ctx,
		`SELECT
			record_id, name, description, sha256, metadata, created_at, modified_at,
			uploader_name, uploader_orcid, is_new_entry, version, current_version
		FROM (
			-- New entries (pending_review status in main table)
			SELECT
				r.id as record_id,
				r.name,
				r.description,
				r.sha256,
				r.metadata,
				r.created_at,
				r.modified_at,
				r.uploader_name,
				r.uploader_orcid,
				true as is_new_entry,
				0 as version,
				'' as current_version
			FROM records r
			WHERE r.moderation_status = $1

			UNION ALL

			-- Latest pending version per record (in history table)
			SELECT
				rh.record_id,
				rh.name,
				rh.description,
				rh.sha256,
				rh.metadata,
				rh.created_at,
				rh.modified_at,
				rh.uploader_name,
				rh.uploader_orcid,
				false as is_new_entry,
				rh.version,
				r.name as current_version
			FROM record_history rh
			JOIN records r ON rh.record_id = r.id
			WHERE rh.moderation_status = 'pending'
			AND rh.change_type = 'PENDING_VERSION'
			AND rh.version = (
				SELECT MAX(version)
				FROM record_history rh2
				WHERE rh2.record_id = rh.record_id
				AND rh2.moderation_status = 'pending'
				AND rh2.change_type = 'PENDING_VERSION'
			)
		) AS pending_items
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		string(StatusPendingReview), limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []PendingItem
	for rows.Next() {
		var item PendingItem
		var version int
		var currentVersion string
		err := rows.Scan(
			&item.RecordID,
			&item.Name,
			&item.Description,
			&item.Sha256,
			&item.Metadata,
			&item.CreatedAt,
			&item.ModifiedAt,
			&item.UploaderName,
			&item.UploaderOrcid,
			&item.IsNewEntry,
			&version,
			&currentVersion,
		)
		if err != nil {
			return nil, 0, err
		}

		if !item.IsNewEntry {
			item.Version = version
			item.CurrentVersion = currentVersion
		}

		// Load categories
		categories, err := r.categoryRepo.GetRecordCategories(ctx, item.RecordID)
		if err == nil {
			item.Categories = categories
		}

		// Load ROR IDs
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, item.RecordID)
		if err == nil {
			item.RorIds = rorIds
		}

		items = append(items, item)
	}

	return items, totalCount, rows.Err()
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
		`SELECT id, name, description, sha256, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count
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
			&rec.Description,
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
		`INSERT INTO moderation_actions (record_id, admin_orcid, action, reason, version_name, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		action.RecordID, action.AdminOrcid, action.Action, action.Reason, action.VersionName, time.Now(),
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

// ModerationHistoryEntry represents a moderation action with record details
type ModerationHistoryEntry struct {
	ModerationAction
	RecordName string
}

// GetRecentModerationHistory returns recent moderation actions with record names
func (r *PostgresModerationRepository) GetRecentModerationHistory(ctx context.Context, limit int) ([]ModerationHistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ma.id, ma.record_id, ma.admin_orcid, ma.action, ma.reason, ma.created_at,
		        COALESCE(NULLIF(ma.version_name, ''), r.name) as display_name
		 FROM moderation_actions ma
		 LEFT JOIN records r ON ma.record_id = r.id
		 ORDER BY ma.created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ModerationHistoryEntry
	for rows.Next() {
		var entry ModerationHistoryEntry
		var reason sql.NullString
		err := rows.Scan(
			&entry.ID,
			&entry.RecordID,
			&entry.AdminOrcid,
			&entry.Action,
			&reason,
			&entry.CreatedAt,
			&entry.RecordName,
		)
		if err != nil {
			return nil, err
		}
		if reason.Valid {
			entry.Reason = reason.String
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
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
