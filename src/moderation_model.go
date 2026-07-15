package main

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ModerationStatus int

const (
	StatusPending ModerationStatus = iota
	StatusApproved
	StatusRejected
	StatusDeleted
	StatusFlagged
)

const StatusUnknown ModerationStatus = -1

var moderationStatusName = map[ModerationStatus]string{
	StatusPending:  "pending",
	StatusApproved: "approved",
	StatusRejected: "rejected",
	StatusDeleted:  "deleted",
	StatusFlagged:  "flagged",
}

// ModerationHistory represents an admin action on a record
type ModerationHistory struct {
	ID          int64
	RecordID    string
	AdminOrcid  string
	// NewStatus      ModerationStatus
	// PreviousStatus ModerationStatus
	ModerationStatus      ModerationStatus
	Reason      string
	VersionName string // Name of the version that was moderated
	CreatedAt   time.Time
//	ModifiedAt   time.Time
}

// PendingItem represents an item in the moderation queue
type PendingItem struct {
	RecordID       string          `json:"record_id"`
	Name           string          `json:"name"`
	Description    sql.NullString  `json:"description"`
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
