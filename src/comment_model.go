package main

import (
	"database/sql"
	"time"
)

type Comment struct {
	ID             int64  `json:"id"`
	RecordID       string `json:"record_id"`
	CommenterName  string `json:"commenter_name"`
	CommenterOrcid string `json:"commenter_orcid"`
	Content        string `json:"content"`
	//	ModerationStatus ModerationStatus `json:"moderation_status,omitempty"` ?
	ModerationStatus ModerationStatus `json:"moderation_status"`
	CreatedAt        time.Time        `json:"created_at"`
	ModifiedAt       time.Time        `json:"modified_at"`
}

type CommentModerationHistory struct {
	ID             int64
	CommentID      int64
	AdminOrcid     string
	NewStatus      ModerationStatus
	PreviousStatus ModerationStatus
	Reason         sql.NullString
	CreatedAt      time.Time
	ModifiedAt     time.Time
}
