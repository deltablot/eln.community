package main

import (
	"context"
	"database/sql"
	"fmt"
)

type NotificationService struct {
	adminRepo      AdminRepository
	emailQueueRepo EmailQueueRepository
	commentRepo    CommentRepository
}

func NewNotificationService(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository, commentRepo CommentRepository) *NotificationService {
	return &NotificationService{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
		commentRepo:    commentRepo,
	}
}

func notificationErr(msg string, err error) error {
	return fmt.Errorf("notification service: failed to create notification for %s: %w", msg, err)
}

func buildAdminModerationRequestBody(item string, action string, owner string, content string) string {
	var body string

	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	body = fmt.Sprintf("Hello,\n\nA new %s has been %s by %s to ELN Community and is awaiting moderation.\n%s\nAs an administrator, please review the %s and approve it if it can be shared with the community. If you are unsure or if the %s does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you.", item, action, owner, commentContent, item, item)

	return body
}

func buildApprovedCommentBody(action string, owner string, content string) string {
	var body string

	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	body = fmt.Sprintf("Hello,\n\nA new comment from %s has been %s in ELN Community.\n%s\nIt is now available on the platform and can be shared with the community.\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.", owner, action, commentContent)

	return body
}

func buildModerationBody(item string, status ModerationStatus) string {
	var body string

	switch status {
	case StatusApproved:
		body = fmt.Sprintf("Good news!\nYour %s has been approved by the ELN Community moderation team.\n\nIt is now available on the platform and can be shared with the community.", item)
	case StatusRejected:
		body = fmt.Sprintf("Your %s has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team at contact@deltablot.email.", item)
	}

	return fmt.Sprintf("Hello,\n\n%s\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.", body)
}

func (s *NotificationService) enqueueEmail(ctx context.Context, recordId string, commentId sql.NullInt64, recipientOrcid string, subject string, body string) error {
	if s.emailQueueRepo == nil {
		return fmt.Errorf("notification service: emailQueueRepo is nil")
	}

	item := &EmailQueue{
		RecordID:       recordId,
		CommentID:      commentId,
		RecipientOrcid: recipientOrcid,
		Subject:        fmt.Sprintf("ELN Community: %s", subject),
		Body:           body,
	}
	if _, err := s.emailQueueRepo.Enqueue(ctx, item); err != nil {
		return fmt.Errorf("notification service: failed to enqueue notification: %w", err)
	}
	return nil
}

func (s *NotificationService) enqueueForAdmins(ctx context.Context, recordId string, commentId sql.NullInt64, item string, body string) error {
	notifiableAdmins, err := s.adminRepo.GetAllAdmins(ctx)
	if err != nil {
		return fmt.Errorf("notification service: failed to get notifiable admins: %w", err)
	}

	subject := fmt.Sprintf("new %s awaiting moderation", item)
	for _, admin := range notifiableAdmins {
		if err := s.enqueueEmail(ctx, recordId, commentId, admin.Orcid, subject, body); err != nil {
			return notificationErr("admins", err)
		}
	}
	return nil
}

func (s *NotificationService) CreateForRecord(ctx context.Context, record *Record) error {
	body := buildAdminModerationRequestBody("record", "uploaded", record.UploaderName, "")

	if err := s.enqueueForAdmins(ctx, record.Id, sql.NullInt64{Valid: false}, "record", body); err != nil {
		return notificationErr("record uploaded", err)
	}
	return nil
}

func (s *NotificationService) CreateForComment(ctx context.Context, comment *Comment) error {
	body := buildAdminModerationRequestBody("comment", "posted", comment.CommenterName, comment.Content)

	if err := s.enqueueForAdmins(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, "comment", body); err != nil {
		return notificationErr("comment posted", err)
	}
	return nil
}

func (s *NotificationService) CreateForRecordModeration(ctx context.Context, id string, uploaderOrcid string, status ModerationStatus) error {
	body := buildModerationBody("record", status)

	if err := s.enqueueEmail(ctx, id, sql.NullInt64{Valid: false}, uploaderOrcid, "update on your record submission", body); err != nil {
		return notificationErr("record moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error {
	body := buildModerationBody("comment", status)

	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, comment.CommenterOrcid, "update on your comment submission", body); err != nil {
		return notificationErr("comment moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForApprovedComment(ctx context.Context, recipientOrcid string, comment *Comment, subject string, action string) error {
	body := buildApprovedCommentBody(action, comment.CommenterName, comment.Content)

	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, recipientOrcid, subject, body); err != nil {
		return notificationErr("comment approved", err)
	}

	return nil
}
