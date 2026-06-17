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

type EmailBody struct {
	Text string
	HTML string
}

func NewNotificationService(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository, commentRepo CommentRepository) *NotificationService {
	return &NotificationService{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
		commentRepo:    commentRepo,
	}
}

func buildEmailBody(body string) EmailBody {
	return EmailBody{
		Text: body,
		HTML: textToHTML(body),
	}
}

const service = "notification service"

func notificationErr(msg string, err error) error {
	return fmt.Errorf("%s: failed to create notification for %s: %w", service, msg, err)
}

func textToHTML(body string) string {
	escapedText := html.EscapeString(body)

	escapedText = strings.ReplaceAll(escapedText, "https://eln.community", `<a href="https://eln.community" target="_blank">https://eln.community</a>`)

	escapedText = strings.ReplaceAll(escapedText, "contact@deltablot.email", `<a href="mailto:contact@deltablot.email">contact@deltablot.email</a>`)

	escapedText = strings.ReplaceAll(escapedText, "\n", "<br>")
	return fmt.Sprintf(`<!doctype html><html><body style="font-family: Arial, sans-serif; font-size: 14px; line-height: 1.5; color: #222;">%s</body></html>`, escapedText)
}

func buildAdminModerationRequestBodyText(item string, action string, owner string, content string) EmailBody {
	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	body := fmt.Sprintf("Hello,\n\nA new %s has been %s by %s to ELN Community and is awaiting moderation.\n%s\nAs an administrator, please review the %s and approve it if it can be shared with the community. If you are unsure or if the %s does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you.", item, action, owner, commentContent, item, item)

	return buildEmailBody(body)
}

func buildApprovedCommentBody(action string, owner string, content string) EmailBody {
	var commentContent string
	if len(content) > 0 {
		commentContent = fmt.Sprintf("See the comment below:\n\"%s\"\n", content)
	}
	body := fmt.Sprintf("Hello,\n\nA new comment from %s has been %s in ELN Community.\n%s\nIt is now available on the platform and can be shared with the community.\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.", owner, action, commentContent)

	return buildEmailBody(body)
}

func buildModerationBody(item string, status ModerationStatus) EmailBody {
	var body string

	switch status {
	case StatusApproved:
		body = fmt.Sprintf("Good news!\nYour %s has been approved by the ELN Community moderation team.\n\nIt is now available on the platform and can be shared with the community.", item)
	case StatusRejected:
		body = fmt.Sprintf("Your %s has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team at contact@deltablot.email.", item)
	}

	fullBody := fmt.Sprintf("Hello,\n\n%s\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.", body)
	return buildEmailBody(fullBody)
}

func (s *NotificationService) enqueueEmail(ctx context.Context, recordId string, commentId sql.NullInt64, recipientOrcid string, subject string, bodyText string, bodyHTML string) error {
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
		BodyText:       bodyText,
		BodyHTML:       bodyHTML,
	}
	if _, err := s.emailQueueRepo.Enqueue(ctx, item); err != nil {
		return fmt.Errorf("%s: failed to enqueue notification: %w", service, err)
	}
	return nil
}

func (s *NotificationService) enqueueForAdmins(ctx context.Context, recordId string, commentId sql.NullInt64, item string, bodyText string, bodyHTML string) error {
	notifiableAdmins, err := s.adminRepo.GetAllAdmins(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get notifiable admins: %w", service, err)
	}

	subject := fmt.Sprintf("new %s awaiting moderation", item)
	for _, admin := range notifiableAdmins {
		if err := s.enqueueEmail(ctx, recordId, commentId, admin.Orcid, subject, bodyText, bodyHTML); err != nil {
			return notificationErr("admins", err)
		}
	}
	return nil
}

func (s *NotificationService) CreateForRecord(ctx context.Context, record *Record) error {
	body := buildAdminModerationRequestBodyText("record", "uploaded", record.UploaderName, "")
	bodyText := body.Text
	bodyHTML := body.HTML

	if err := s.enqueueForAdmins(ctx, record.Id, sql.NullInt64{Valid: false}, "record", bodyText, bodyHTML); err != nil {
		return notificationErr("record uploaded", err)
	}
	return nil
}

func (s *NotificationService) CreateForComment(ctx context.Context, comment *Comment) error {
	body := buildAdminModerationRequestBodyText("comment", "posted", comment.CommenterName, comment.Content)
	bodyText := body.Text
	bodyHTML := body.HTML

	if err := s.enqueueForAdmins(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, "comment", bodyText, bodyHTML); err != nil {
		return notificationErr("comment posted", err)
	}
	return nil
}

func (s *NotificationService) CreateForRecordModeration(ctx context.Context, id string, uploaderOrcid string, status ModerationStatus) error {
	body := buildModerationBody("record", status)
	bodyText := body.Text
	bodyHTML := body.HTML

	if err := s.enqueueEmail(ctx, id, sql.NullInt64{Valid: false}, uploaderOrcid, "update on your record submission", bodyText, bodyHTML); err != nil {
		return notificationErr("record moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error {
	body := buildModerationBody("comment", status)
	bodyText := body.Text
	bodyHTML := body.HTML

	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, comment.CommenterOrcid, "update on your comment submission", bodyText, bodyHTML); err != nil {
		return notificationErr("comment moderation", err)
	}

	return nil
}

func (s *NotificationService) CreateForApprovedComment(ctx context.Context, recipientOrcid string, comment *Comment, subject string, action string) error {
	body := buildApprovedCommentBody(action, comment.CommenterName, comment.Content)
	bodyText := body.Text
	bodyHTML := body.HTML

	if err := s.enqueueEmail(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, recipientOrcid, subject, bodyText, bodyHTML); err != nil {
		return notificationErr("comment approved", err)
	}

	return nil
}
