package main

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"strings"
)

type NotificationService struct {
	adminRepo      AdminRepository
	emailQueueRepo EmailQueueRepository
	commentRepo    CommentRepository
}

type EmailContent struct {
	Text    string
	HTML    string
	Subject string
}

func NewNotificationService(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository, commentRepo CommentRepository) *NotificationService {
	return &NotificationService{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
		commentRepo:    commentRepo,
	}
}

func buildEmailContent(content string) EmailContent {
	return EmailContent{
		Text:    content,
		HTML:    textToHTML(content),
		Subject: content,
	}
}

const service = "Error: notification service"

func notificationErr(msg string, err error) error {
	return fmt.Errorf("%s: failed to create notification for %s: %w", service, msg, err)
}

func textToHTML(body string) string {
	escapedText := html.EscapeString(body)
	replacer := strings.NewReplacer("https://eln.community/moderation", `<a href="https://eln.community/moderation" target="_blank" rel="noopener noreferrer">https://eln.community/moderation</a>`, "https://eln.community", `<a href="https://eln.community" target="_blank" rel="noopener noreferrer">https://eln.community</a>`, "contact@deltablot.email", `<a href="mailto:contact@deltablot.email">contact@deltablot.email</a>`, "\n", "<br>")

	escapedText = replacer.Replace(escapedText)

	return fmt.Sprintf(`<!doctype html><html><body style="font-family: Arial, sans-serif; font-size: 14px; line-height: 1.5; color: #222;">%s</body></html>`, escapedText)
}

func buildAdminModerationRequestBodyText(item string, action string, owner string, content string, status ModerationStatus) EmailContent {
	var actionRequired string
	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	var introduction = fmt.Sprintf("Hello,\n\nA new %s has been %s by %s to ELN Community and is awaiting moderation.\n%s\nAs an administrator, please review the %s and", item, action, owner, commentContent, item)

	var closing = fmt.Sprintf("If you are unsure or if the %s does not meet the platform requirements, you can reject or delete it.\nOpen ELN Community: https://eln.community/moderation\n\nThank you.", item)
	switch status {
	case StatusPending:
		actionRequired = fmt.Sprintf("approve it if it can be shared with the community.")
	case StatusFlagged:
		actionRequired = fmt.Sprintf("decide whether any moderation action is needed.")
	}
	body := fmt.Sprintf("%s %s %s", introduction, actionRequired, closing)
	return buildEmailContent(body)
}

func buildApprovedCommentBody(action string, owner string, content string) EmailContent {
	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	body := fmt.Sprintf("Hello,\n\nA new comment from %s has been %s in ELN Community.\n%s\nIt is now available on the platform and can be shared with the community.\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.", owner, action, commentContent)

	return buildEmailContent(body)
}

func buildModerationBody(item string, status ModerationStatus) EmailContent {
	var body string
	var link string

	switch status {
	case StatusApproved:
		body = fmt.Sprintf("Good news!\nYour %s has been approved by the ELN Community moderation team.\n\nIt is now available on the platform and can be shared with the community.", item)
		link = "\n\nYou can view it here: https://eln.community"
	case StatusRejected:
		body = fmt.Sprintf("Your %s has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team at contact@deltablot.email.", item)
		link = ""
	}

	return buildEmailContent(fmt.Sprintf("Hello,\n\n%s%s\n\nThank you for contributing to open science.", body, link))
}

func (s *NotificationService) enqueueEmailbis(ctx context.Context, recordId string, commentId sql.NullInt64, recipientOrcid string, subject string, body EmailContent) error {
	if s.emailQueueRepo == nil {
		return fmt.Errorf("%s: emailQueueRepo is nil", service)
	}

	if strings.TrimSpace(recipientOrcid) == "" {
		return fmt.Errorf("%s: recipient ORCID is empty", service)
	}

	item := &EmailQueue{
		RecordID:       recordId,
		CommentID:      commentId,
		RecipientOrcid: recipientOrcid,
		Subject:        fmt.Sprintf("ELN Community: %s", subject),
		BodyText:       body.Text,
		BodyHTML:       body.HTML,
	}
	if _, err := s.emailQueueRepo.Enqueue(ctx, item); err != nil {
		return fmt.Errorf("%s: failed to enqueue notification: %w", service, err)
	}
	return nil
}

func (s *NotificationService) enqueueEmail(ctx context.Context, recordId string, commentId sql.NullInt64, recipientOrcid string, body EmailContent) error {
	if s.emailQueueRepo == nil {
		return fmt.Errorf("%s: emailQueueRepo is nil", service)
	}

	if strings.TrimSpace(recipientOrcid) == "" {
		return fmt.Errorf("%s: recipient ORCID is empty", service)
	}

	item := &EmailQueue{
		RecordID:       recordId,
		CommentID:      commentId,
		RecipientOrcid: recipientOrcid,
		Subject:        fmt.Sprintf("ELN Community: %s", body.Subject),
		BodyText:       body.Text,
		BodyHTML:       body.HTML,
	}
	if _, err := s.emailQueueRepo.Enqueue(ctx, item); err != nil {
		return fmt.Errorf("%s: failed to enqueue notification: %w", service, err)
	}
	return nil
}

func (s *NotificationService) enqueueForAdmins(ctx context.Context, recordId string, commentId sql.NullInt64, item string, body EmailContent) error {
	if s.adminRepo == nil {
		return fmt.Errorf("%s: adminRepo is nil", service)
	}
	notifiableAdmins, err := s.adminRepo.GetAllAdmins(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get notifiable admins: %w", service, err)
	}

	body.Subject = fmt.Sprintf("new %s awaiting moderation", item)
	for _, admin := range notifiableAdmins {
		if err := s.enqueueEmail(ctx, recordId, commentId, admin.Orcid, body); err != nil {
			return notificationErr("admins", err)
		}
	}
	return nil
}

func (s *NotificationService) CreateForRecord(ctx context.Context, record *Record) error {
	if record == nil {
		return fmt.Errorf("%s: record is nil", service)
	}
	body := buildAdminModerationRequestBodyText("record", "uploaded", record.UploaderName, "", record.ModerationStatus)

	if err := s.enqueueForAdmins(ctx, record.Id, sql.NullInt64{Valid: false}, "record", body); err != nil {
		return notificationErr("record uploaded", err)
	}
	return nil
}

func (s *NotificationService) CreateForComment(ctx context.Context, comment *Comment, status ModerationStatus) error {
	if comment == nil {
		return fmt.Errorf("%s: comment is nil", service)
	}
	action := "posted"
	if status == StatusFlagged {
		action = "reported"
	}
	body := buildAdminModerationRequestBodyText("comment", action, comment.CommenterName, comment.Content, status)

	if err := s.enqueueForAdmins(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, "comment", body); err != nil {
		return notificationErr("comment posted", err)
	}
	return nil
}

func (s *NotificationService) CreateForRecordModeration(ctx context.Context, id string, recipientOrcid string, status ModerationStatus) error {
	body := buildModerationBody("record", status)

	body.Subject = "update on your record submission"
	if err := s.enqueueEmail(ctx, id, sql.NullInt64{Valid: false}, recipientOrcid, body); err != nil {
		return notificationErr("record moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error {
	if comment == nil {
		return fmt.Errorf("%s: comment is nil", service)
	}
	body := buildModerationBody("comment", status)

	body.Subject = "update on your comment submission"
	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, comment.CommenterOrcid, body); err != nil {
		return notificationErr("comment moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForApprovedComment(ctx context.Context, recipientOrcid string, comment *Comment, subject string, action string) error {
	if comment == nil {
		return fmt.Errorf("%s: comment is nil", service)
	}
	body := buildApprovedCommentBody(action, comment.CommenterName, comment.Content)

	body.Subject = subject
	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, recipientOrcid, body); err != nil {
		return notificationErr("comment approved", err)
	}

	return nil
}
