package main

import (
	"encoding/json"
	"time"
)

type Record struct {
	CreatedAt time.Time       `json:"created_at"`
	Id        string          `json:"id"`
	Metadata  json.RawMessage `json:"metadata"`
	// This will be ignored by json.Marshal
	MetadataPretty   string           `json:"-"`
	ModifiedAt       time.Time        `json:"modified_at"`
	Name             string           `json:"name"`
	Sha256           string           `json:"sha256"`
	UploaderName     string           `json:"uploader_name"`
	UploaderOrcid    string           `json:"uploader_orcid"`
	RorIds           []string         `json:"rors,omitempty"`
	Categories       []Category       `json:"categories,omitempty"`
	DownloadCount    int              `json:"download_count"`
	ModerationStatus ModerationStatus `json:"moderation_status,omitempty"`
	License          string           `json:"license"`
	ArchivedAt       *time.Time       `json:"archived_at,omitempty"`
	ArchiveReason    string           `json:"archive_reason,omitempty"`
}

// IsArchived returns true if the record has been archived
func (r *Record) IsArchived() bool {
	return r.ArchivedAt != nil
}

// RecordHistory represents a historical version of a record
type RecordHistory struct {
	HistoryId        int64            `json:"history_id"`
	RecordId         string           `json:"record_id"`
	Version          int              `json:"version"`
	S3Key            string           `json:"-"`
	Name             string           `json:"name"`
	Sha256           string           `json:"sha256"`
	Metadata         json.RawMessage  `json:"metadata"`
	UploaderName     string           `json:"uploader_name"`
	UploaderOrcid    string           `json:"uploader_orcid"`
	DownloadCount    int              `json:"download_count"`
	CreatedAt        time.Time        `json:"created_at"`
	ModifiedAt       time.Time        `json:"modified_at"`
	ArchivedAt       time.Time        `json:"archived_at"`
	ChangeType       string           `json:"change_type"`
	ModerationStatus ModerationStatus `json:"moderation_status,omitempty"`
	License          string           `json:"license"`
}

type Category struct {
	Id            int64      `json:"id"`
	Name          string     `json:"name"`
	ParentId      *int64     `json:"parent_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ModifiedAt    time.Time  `json:"modified_at"`
	Subcategories []Category `json:"subcategories,omitempty"`
}

type User struct {
	Name  string
	Orcid string
}

type App struct {
	BuildId     string
	MaxFileSize int64
	Version     string
}

type RecordPageData struct {
	App
	Record         Record
	CanEdit        bool
	CanArchive     bool
	IsArchived     bool
	User           *User
	CurrentPage    string
	IsHistorical   bool
	HistoryVersion int
}
