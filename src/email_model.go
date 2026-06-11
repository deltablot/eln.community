package main

import (
	"database/sql"
	"time"
)

type EmailStatus int

const (
	PendingStatus EmailStatus = iota
	ProcessingStatus
	SentStatus
	FailedStatus
)

var statusName = map[EmailStatus]string{
	PendingStatus:    "pending",
	ProcessingStatus: "processing",
	SentStatus:       "sent",
	FailedStatus:     "failed",
}

type EmailQueue struct {
	Id             int64          `json:"id"`
	RecordID       string         `json:"record_id"`
	CommentID      sql.NullInt64  `json:"comment_id"`
	RecipientOrcid string         `json:"recipient_orcid"`
	Subject        string         `json:"subject"`
	Body           string         `json:"body"`
	Status         EmailStatus    `json:"status"`
	Attempts       int            `json:"attempts"`
	LastError      sql.NullString `json:"last_error"`
	CreatedAt      time.Time      `json:"created_at"`
	ModifiedAt     time.Time      `json:"modified_at"`
	SentAt         sql.NullTime   `json:"sent_at"`
}
