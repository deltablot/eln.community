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
	for _, admin := range notifiableAdmins {
		item := &EmailQueue{
			RecordID:         record.Id,
			CommentID:        sql.NullInt64{Valid: false},
			RecipientOrcid:   admin.Orcid,
			Subject:          "ELN Community: new record awaiting moderation",
			Body:             "Hello,\n\nA new record has been uploaded to ELN Community and is awaiting moderation.\n\nAs an administrator, please review the record and approve it if it can be shared with the community. If you are unsure or if the record does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you.",
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

	var body string

	switch status {
	case StatusApproved:
		body = "Good news!\nYour record has been approved by the ELN Community moderation team.\n\nIt is now avalaible on the plaform and can be shared with the community."
	case StatusRejected:
		body = "Your record has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team."
	}

	item := &EmailQueue{
		RecordID:       id,
		CommentID:      sql.NullInt64{Valid: false},
		RecipientOrcid: uploaderOrcid,
		Subject:        "ELN Community: update on your record submission",
		Body:           "Hello,\n\n" + body + "\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.",
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
	for _, admin := range notifiableAdmins {
		item := &EmailQueue{
			RecordID: comment.RecordID,
			CommentID: sql.NullInt64{
				Int64: comment.ID,
				Valid: true,
			},
			RecipientOrcid: admin.Orcid,
			Subject:        "ELN Community: new comment awaiting moderation",
			Body:           "Hello,\n\nA new comment has been posted by " + comment.CommenterName + " to ELN Community and is awaiting moderation.\nSee the comment below:\n\"" + comment.Content + "\"\n\nAs an administrator, please review the comment and approve it if it can be shared with the community. If you are unsure or if the comment does not meet the platform requirements, you can reject it.\nOpen ELN Community: https://eln.community\n\nThank you.",
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

	var body string

	switch status {
	case StatusApproved:
		body = "Good news!\nYour comment has been approved by the ELN Community moderation team.\n\nIt is now avalaible on the plaform and can be shared with the community."
	case StatusRejected:
		body = "Your comment has been reviewed by the ELN Community moderation team and was not approved for publication.\n\nIf you think this is a mistake or need more information, please contact the ELN Community team."
	}

	item := &EmailQueue{
		RecordID: comment.RecordID,
		CommentID: sql.NullInt64{
			Int64: comment.ID,
			Valid: true,
		},
		RecipientOrcid: comment.CommenterOrcid,
		Subject:        "ELN Community: update on your record submission",
		Body:           "Hello,\n\n" + body + "\n\nYou can view it here: https://eln.community\n\nThank you for contributing to open science.",
	}

	_, err := s.emailQueueRepo.Enqueue(ctx, item)
	if err != nil {
		log.Printf("failed to enqueue email notification: %v", err)
	} else {
		log.Printf("\nEnqueue email notification success\n")
	}

	return nil
}
