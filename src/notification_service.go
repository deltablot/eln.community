package main

import (
	"context"
	"database/sql"
	"log"
)

type NotificationService struct {
	adminRepo      AdminRepository
	emailQueueRepo EmailQueueRepository
}

type NotificationRepositoty interface {
	CreateRecordNotification(ctx context.Context, record *Record) error
}

func NewNotificationService(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository) *NotificationService {
	return &NotificationService{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
	}
}

func (s *NotificationService) CreateRecordNotification(ctx context.Context, record *Record) error {
	notifiableAdmins, err := s.adminRepo.GetNotifiableAdmins(ctx)
	if err != nil {
		log.Printf("failed to get notifiable admins: %v", err)
		return err
	}
//	log.Printf("notifiable admins: %+v", notifiableAdmins)

	if s.emailQueueRepo == nil {
		log.Printf("emailQueueRepo is nil")
		return nil
	}
	for _, admin := range notifiableAdmins {
		item := &EmailQueue{
			RecordID:         record.Id,
			CommentID:        sql.NullInt64{Valid: false},
			RecipientOrcid:   admin.Orcid,
			Subject:          "Test notification",
			Body:             "Test body",
			RecipientType:    AdminRecipient,
			NotificationType: RecordCreatedAdminNotif,
		}

		queuedItem, err := s.emailQueueRepo.Enqueue(ctx, item)
		if err != nil {
			log.Printf("failed to enqueue email notification: %v", err)
		} else {
			log.Printf("Enqueue email notification success: %v", queuedItem)
		}
	}

	return nil
}
