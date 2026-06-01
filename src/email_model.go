package main

import (
	"database/sql"
	"time"
)

type EmailStatus string

const (
	PendingStatus EmailStatus = "pending"
	SentStatus    EmailStatus = "sent"
	FailedStatus  EmailStatus = "failed"
)

type EmailRecipientType string

const (
	AdminRecipient EmailRecipientType = "admin_recipient"
	RecordOwner    EmailRecipientType = "record_owner"
	Commenter      EmailRecipientType = "commenter"
)

type EmailNotificationType string

const (
	RecordCreatedAdminNotif  EmailNotificationType = "record_created_admin_notif"
	CommentCreatedAdminNotif EmailNotificationType = "comment_created_admin_notif"
	RecordApprovedNotif      EmailNotificationType = "record_approved_notif"
	RecordRejectedNotif      EmailNotificationType = "record_rejected_notif"
	CommentApprovedNotif     EmailNotificationType = "comment_approved_notif"
	CommentRejectedNotif     EmailNotificationType = "comment_rejected_notif"
)

type EmailQueue struct {
	Id               int64                 `json:"id"`
	RecordID         string                `json:"record_id"`
	CommentID        sql.NullInt64         `json:"comment_id"`
	RecipientOrcid   string                `json:"recipient_orcid"`
	SendFrom         string                `json:"send_from"`
	Subject          string                `json:"subject"`
	Body             string                `json:"body"`
	RecipientType    EmailRecipientType    `json:"recipient_type"`
	NotificationType EmailNotificationType `json:"notification_type"`
	Status           EmailStatus           `json:"status"`
	Attempts         int                   `json:"attempts"`
	LastError        sql.NullString        `json:"last_error"`
	CreatedAt        time.Time             `json:"created_at"`
	SentAt           sql.NullTime          `json:"sent_at"`
}
