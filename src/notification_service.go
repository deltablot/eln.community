package main

import (
	"context"
	"database/sql"
	"log"
)

type NotificationService struct {
	adminRepo      AdminRepository
	emailQueueRepo EmailQueueRepository
	commentRepo    CommentRepository
}

type NotificationCreator interface {
	CreateRecord(ctx context.Context, record *Record) error
	CreateRecordModeration(ctx context.Context, id string, uploaderOrcid string, notifType string) error
	CreateComment(ctx context.Context, comment *Comment) error
	CreateCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error
}

func NewNotificationService(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository, commentRepo CommentRepository) *NotificationService {
	return &NotificationService{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
		commentRepo:    commentRepo,
	}
}

func displayBodyCreation(item string, action string, owner string, content string) string {
	var body string

	var commentContent string
	if len(content) > 0 {
		commentContent = "See the comment below:\n\"" + content + "\"\n"
	}
	body = "Hello\n\nA new " + item + " has been " + action + " by " + owner + " to ELN Community and is awaiting moderation.\n" + commentContent + "\nAs an administrator, please review the comment and approve it if it can be shared with the community. If you are unsure or if the comment does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you."

	return body
}

func displayBodyModeration(item string, status ModerationStatus) string {
	var body string

	switch status {
	case StatusApproved:
		body = "Good news!\nYour " + item + " has been approved by the ELN Community moderation team.\n\nIt is now avalaible on the plaform and can be shared with the community."
	case StatusRejected:
		body = "Your " + item + " has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team."
	}

	return "Hello,\n\n" + body + "\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science."
}

func (s *NotificationService) CreateRecord(ctx context.Context, record *Record) error {
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

	body := displayBodyCreation("record", "uploaded", record.UploaderName, "")
	for _, admin := range notifiableAdmins {
		item := &EmailQueue{
			RecordID:         record.Id,
			CommentID:        sql.NullInt64{Valid: false},
			RecipientOrcid:   admin.Orcid,
			Subject:          "ELN Community: new record awaiting moderation",
			Body:             body,
			RecipientType:    AdminRecipient,
			NotificationType: RecordCreatedAdminNotif,
		}

		_, err := s.emailQueueRepo.Enqueue(ctx, item)
		if err != nil {
			log.Printf("failed to enqueue email notification: %v", err)
		} else {
			log.Printf("\n\nEnqueue email notification success\n")
		}
	}

	return nil
}

func (s *NotificationService) CreateRecordModeration(ctx context.Context, id string, uploaderOrcid string, status ModerationStatus) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := displayBodyModeration("record", status)

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

func (s *NotificationService) CreateComment(ctx context.Context, comment *Comment) error {
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
	body := displayBodyCreation("comment", "posted", comment.CommenterName, comment.Content)
	for _, admin := range notifiableAdmins {
		item := &EmailQueue{
			RecordID: comment.RecordID,
			CommentID: sql.NullInt64{
				Int64: comment.ID,
				Valid: true,
			},
			RecipientOrcid: admin.Orcid,
			Subject:        "ELN Community: new comment awaiting moderation",
			Body:           body,
		}

		_, err := s.emailQueueRepo.Enqueue(ctx, item)
		if err != nil {
			log.Printf("failed to enqueue email notification: %v", err)
		} else {
			log.Printf("\n\nEnqueue email notification success\n")
		}
	}

	return nil
}

func (s *NotificationService) CreateCommentModeration(ctx context.Context, comment *Comment, status ModerationStatus) error {
	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}

	body := displayBodyModeration("comment", status)

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
