package main

import (
	"time"
)

type Comment struct {
	ID               int64            `json:"id"`
	RecordID         string           `json:"record_id"`
	CommenterName    string           `json:"commenter_name"`
	CommenterOrcid   string           `json:"commenter_orcid"`
	Content          string           `json:"content"`
//	ModerationStatus ModerationStatus `json:"moderation_status,omitempty"` ?
	ModerationStatus ModerationStatus `json:"moderation_status"`
	CreatedAt        time.Time        `json:"created_at"`
	ModifiedAt       time.Time        `json:"modified_at"`
}

// TODO:
//       * Renommer cette table CommentModerationHistory
//       * virer action -> new_status
//       * ajouter previous_status
//       * ajouter modified_at
//       * Mettre reason en sql.NullString ou en string mais permettre le NULL
type CommentModerationAction struct {
	ID         int64
	CommentID  int64
	AdminOrcid string
	Action     ModerationStatus
	Reason     string
	CreatedAt  time.Time
}
