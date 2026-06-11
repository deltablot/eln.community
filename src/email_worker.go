package main

import (
	"context"
	"fmt"
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

const worker = "email worker"
const MAX_ATTEMPTS = 3

func (w *EmailWorker) ProcessPending(ctx context.Context, limit int) error {
	pendingEmails, err := w.emailQueueRepo.GetPending(ctx, limit)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch pending emails: %w", worker, err)
	}

	for _, pending := range pendingEmails {
		recipientEmail, err := w.orcidService.GetEmail(ctx, pending.RecipientOrcid)
		if err != nil {
			if pending.Attempts+1 < MAX_ATTEMPTS {
				markErr := w.emailQueueRepo.MarkForRetry(ctx, pending.Id, err.Error())
				if markErr != nil {
					return fmt.Errorf("%s: failed to mark email as pending for retry (queue_id %d) after recipient email resolution failure: %w", worker, pending.Id, markErr)
				}
			} else {
				markErr := w.emailQueueRepo.MarkAsFailed(ctx, pending.Id, err.Error())
				if markErr != nil {
					return fmt.Errorf("%s: failed to mark email as failed (queue_id %d) after recipient email resolution failure: %w", worker, pending.Id, markErr)
				}
			}
			continue
		}

		err = w.emailSender.Send(recipientEmail, pending.Subject, pending.Body)

		if err != nil {
			markErr := w.emailQueueRepo.MarkAsFailed(ctx, pending.Id, err.Error())
			if markErr != nil {
				return fmt.Errorf("%s: failed to mark email as failed (queue_id %d) after send failure: %w", worker, pending.Id, markErr)
			}
			continue
		}

		markErr := w.emailQueueRepo.MarkAsSent(ctx, pending.Id)
		if markErr != nil {
			return fmt.Errorf("%s: failed to mark email as sent (queue_id %d): %w", worker, pending.Id, markErr)
		}
		log.Printf("%s: email sent successfully", worker)
	}
	return nil
}
