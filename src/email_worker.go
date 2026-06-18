package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/textproto"
)

type EmailWorker struct {
	emailQueueRepo EmailQueueRepository
	emailSender    *EmailSender
}

func NewEmailWorker(emailQueueRepo EmailQueueRepository, emailSender *EmailSender) *EmailWorker {
	return &EmailWorker{
		emailQueueRepo: emailQueueRepo,
		emailSender:    emailSender,
	}
}

const worker = "email worker"
const maxAttempts = 3

func isSMTPPermanentErr(err error) bool {
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		return smtpErr.Code >= 500
	}
	return false
}

func (w *EmailWorker) failed(ctx context.Context, pending EmailQueue, reason string, err error) error {
	markErr := w.emailQueueRepo.MarkAsFailed(ctx, pending.Id, err.Error())
	if markErr != nil {
		return fmt.Errorf("%s: failed to mark email as failed (queue_id %d) after %s failure: %w", worker, pending.Id, reason, markErr)
	}
	return nil
}

func (w *EmailWorker) retry(ctx context.Context, pending EmailQueue, reason string, err error) error {
	markErr := w.emailQueueRepo.MarkForRetry(ctx, pending.Id, err.Error())
	if markErr != nil {
		return fmt.Errorf("%s: failed to mark email as pending for retry (queue_id %d) after %s failure: %w", worker, pending.Id, reason, markErr)
	}
	return nil
}

func (w *EmailWorker) retryOrFail(ctx context.Context, pending EmailQueue, reason string, err error) error {
	var emailUnavailable *EmailUnavailable
	if errors.As(err, &emailUnavailable) || isSMTPPermanentErr(err) {
		return w.failed(ctx, pending, reason, err)
	}
	if pending.Attempts+1 < maxAttempts {
		return w.retry(ctx, pending, reason, err)
	}

	return w.failed(ctx, pending, reason, err)
}

func (w *EmailWorker) ProcessPending(ctx context.Context, limit int) error {
	pendingEmails, err := w.emailQueueRepo.GetPending(ctx, limit)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch pending emails: %w", worker, err)
	}

	for _, pending := range pendingEmails {
		recipientEmail, err := GetEmail(ctx, pending.RecipientOrcid)
		if err != nil {
			if markErr := w.retryOrFail(ctx, pending, "recipient email resolution", err); markErr != nil {
				return markErr
			}
			continue
		}

		err = w.emailSender.Send(recipientEmail, pending.Subject, pending.BodyText, pending.BodyHTML)
		if err != nil {
			if markErr := w.retryOrFail(ctx, pending, "send", err); markErr != nil {
				return markErr
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
