package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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

func buildCreationBody(item string, action string, owner string, content string) string {
	var body string

	var commentContent string
	if len(content) > 0 {
		commentContent = "See the comment below:\n\"" + content + "\"\n"
	}
	body = "Hello\n\nA new " + item + " has been " + action + " by " + owner + " to ELN Community and is awaiting moderation.\n" + commentContent + "\nAs an administrator, please review the " + item + "  and approve it if it can be shared with the community. If you are unsure or if the " + item + " does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you."

	return body
}

func buildModerationBody(item string, status ModerationStatus) string {
	var body string

	switch status {
	case StatusApproved:
		body = "Good news!\nYour " + item + " has been approved by the ELN Community moderation team.\n\nIt is now avalaible on the plaform and can be shared with the community."
	case StatusRejected:
		body = "Your " + item + " has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team at <TODO: ADD ADDRESS>."
	}

	return "Hello,\n\n" + body + "\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science."
}

func (s *NotificationService) handleData(ctx context.Context, recordId string, commentId sql.NullInt64, recipient string, subject string, body string) error {
	item := &EmailQueue{
		RecordID:       recordId,
		CommentID:      commentId,
		RecipientOrcid: recipient,
		Subject:        subject,
		Body:           body,
	}
	if _, err := s.emailQueueRepo.Enqueue(ctx, item); err != nil {
		return fmt.Errorf("Notification Service: failed to enqueue admin notification: %w", err)
	}
	return nil
}

func (s *NotificationService) createAdmin(ctx context.Context, recordId string, commentId sql.NullInt64, subject string, body string) error {
	notifiableAdmins, err := s.adminRepo.GetAllAdmins(ctx)
	if err != nil {
		log.Printf("failed to get notifiable admins: %v", err)
		return err
	}
	log.Printf("notifiable admins: %+v", notifiableAdmins)

	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}
	for _, admin := range notifiableAdmins {
		s.handleData(ctx, recordId, commentId, admin.Orcid, subject, body)
	}
	return nil
}

func (s *NotificationService) CreateRecord(ctx context.Context, record *Record) error {
	body := buildCreationBody("record", "uploaded", record.UploaderName, "")

	return s.createAdmin(ctx, record.Id, sql.NullInt64{Valid: false}, "ELN Community: new record awaiting moderation", body)
}

func (s *NotificationService) CreateComment(ctx context.Context, comment *Comment) error {
	body := buildCreationBody("comment", "posted", comment.CommenterName, comment.Content)

	return s.createAdmin(ctx, comment.RecordID, sql.NullInt64{Int64: comment.ID, Valid: true}, "ELN Community: new record awaiting moderation", body)
}

func (s *NotificationService) CreateRecordModeration(ctx context.Context, id string, uploaderOrcid string, status ModerationStatus) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := buildModerationBody("record", status)

	item := &EmailQueue{
		RecordID:       id,
		CommentID:      sql.NullInt64{Valid: false},
		RecipientOrcid: uploaderOrcid,
		Subject:        "ELN Community: update on your record submission",
		Body:           body,
	}

	_, err := s.emailQueueRepo.Enqueue(ctx, item)
	if err != nil {
		log.Printf("failed to enqueue email notification: %v", err)
	} else {
		log.Printf("\nEnqueue email notification success\n")
	}

	return nil
}

func (s *NotificationService) CreateCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := buildModerationBody("comment", status)

	item := &EmailQueue{
		RecordID: comment.RecordID,
		CommentID: sql.NullInt64{
			Int64: comment.ID,
			Valid: true,
		},
		RecipientOrcid: comment.CommenterOrcid,
		Subject:        "ELN Community: update on your comment submission",
		Body:           body,
	}

	_, err := s.emailQueueRepo.Enqueue(ctx, item)
	if err != nil {
		log.Printf("failed to enqueue email notification: %v", err)
	} else {
		log.Printf("\nEnqueue email notification success\n")
	}

	return nil
}

func (s *NotificationService) CreateCommentOwner(ctx context.Context, recordOwner string, comment *Comment) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := buildCreationBody("comment", "posted on your record", comment.CommenterName, comment.Content)

	item := &EmailQueue{
		RecordID: comment.RecordID,
		CommentID: sql.NullInt64{
			Int64: comment.ID,
			Valid: true,
		},
		RecipientOrcid: recordOwner,
		Subject:        "ELN Community: a new comment has been posted on your record",
		Body:           body,
	}

	_, err := s.emailQueueRepo.Enqueue(ctx, item)
	if err != nil {
		log.Printf("failed to enqueue email notification: %v", err)
	} else {
		log.Printf("\nEnqueue email notification success\n")
	}

	return nil
}

func (s *NotificationService) CreateOtherCommentator(ctx context.Context, commentator string, comment *Comment) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := buildCreationBody("comment", "posted on a record you previously commented on", comment.CommenterName, comment.Content)

	item := &EmailQueue{
		RecordID: comment.RecordID,
		CommentID: sql.NullInt64{
			Int64: comment.ID,
			Valid: true,
		},
		RecipientOrcid: commentator,
		Subject:        "ELN Community: new activity on a record you follow",
		Body:           body,
	}

	_, err := s.emailQueueRepo.Enqueue(ctx, item)
	if err != nil {
		log.Printf("failed to enqueue email notification: %v", err)
	} else {
		log.Printf("\nEnqueue email notification success\n")
	}

	return nil
}
