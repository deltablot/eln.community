package main

import (
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
	//Action     ModerationStatus
	Reason     string
	CreatedAt  time.Time
}
