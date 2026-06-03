package main

import (
	"context"
	"log"
)

type EmailWorker struct {
	emailQueueRepo EmailQueueRepository
	emailSender    *EmailSender
	orcidService   *OrcidService
}

func NewEmailWorker(emailQueueRepo EmailQueueRepository, emailSender *EmailSender, orcidService *OrcidService) *EmailWorker {
	return &EmailWorker{
		emailQueueRepo: emailQueueRepo,
		emailSender:    emailSender,
		orcidService:   orcidService,
	}
}

func (w *EmailWorker) ProcessPendingEmails(ctx context.Context, limit int) error {
	pendingEmails, err := w.emailQueueRepo.GetPendingEmails(ctx, limit)
	if err != nil {
		log.Printf("Email worker: failed to fetch pending emails. Error: %v", err)
		return err
	}

	for _, pending := range pendingEmails {
		recipientEmail, err := w.orcidService.GetEmailFromOrcid(ctx, pending.RecipientOrcid)
		if err != nil {
			markErr := w.emailQueueRepo.MarkEmailAsFailed(ctx, pending.Id, err.Error())
			if markErr != nil {
				log.Printf("Email worker: failed to mark email as failed queue_id:%d after resolving recipient email failed. Error: %v", pending.Id, markErr)
				return markErr
			}
			continue
		}

		err = w.emailSender.Send(recipientEmail, pending.Subject, pending.Body)

		if err != nil {
			markErr := w.emailQueueRepo.MarkEmailAsFailed(ctx, pending.Id, err.Error())
			if markErr != nil {
				log.Printf("Email worker: failed to mark email as failed queue_id:%d after send failure. Error: %v", pending.Id, markErr)
				return markErr
			}
			continue
		}

		markErr := w.emailQueueRepo.MarkEmailAsSent(ctx, pending.Id)
		if markErr != nil {
			log.Printf("Email worker: failed to mark email as sent queue_id:%d. Error: %v", pending.Id, markErr)
			return markErr
		}
		log.Printf("Email worker: email sent successfully for email queue_id:%d record: %s", pending.Id, pending.RecordID)
	}
	return nil
}
