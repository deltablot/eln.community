package main

import (
	"fmt"
	"net/textproto"
	"testing"
    "context"
)

const smtpPermanentErr = 530
const smtpTemporaryErr = 450

type MockEmailQueueRepository struct {
	markAsFailedCalled bool
	markForRetryCalled bool
	id                 int64
	lastError          string
}


func (m *MockEmailQueueRepository) Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error) {
	return nil, nil
}

func (m *MockEmailQueueRepository) GetPending(ctx context.Context, limit int) ([]EmailQueue, error) {
	return []EmailQueue{}, nil
}

func (m *MockEmailQueueRepository) MarkAsSent(ctx context.Context, id int64) error {
	return nil
}

func (m *MockEmailQueueRepository) MarkAsFailed(ctx context.Context, id int64, errMsg string) error {
	m.markAsFailedCalled = true
    m.id = id
    m.lastError = errMsg
	return nil
}

func (m *MockEmailQueueRepository) MarkForRetry(ctx context.Context, id int64, errMsg string) error {
	m.markForRetryCalled = true
    m.id = id
    m.lastError = errMsg
	return nil
}

func TestIsSMTPPermanentErr(t *testing.T) {
    err := fmt.Errorf("error: %w", &textproto.Error{
		Code: smtpPermanentErr,
		Msg:  "authentication required",
	})

	got := isSMTPPermanentErr(err)
	if !got {
		t.Fatal("expected SMTP error to be permanent")
	}
}

func TestIsSMTPTemporaryErr(t *testing.T) {
    err := fmt.Errorf("error: %w", &textproto.Error{
		Code: smtpTemporaryErr,
		Msg:  "domain not found",
	})

	got := !isSMTPPermanentErr(err)
	if !got {
		t.Fatal("expected SMTP error to be temporary")
	}
}


func TestRetryOrFailSMTPPermanentError(t *testing.T) {
    ctx := context.Background()

    mockRepo := &MockEmailQueueRepository{}

    worker := &EmailWorker{
        emailQueueRepo: mockRepo,
    }

    pending := EmailQueue{
        Id: 2,
        Attempts: 0,
    }

    smtpErr := fmt.Errorf("error: %w", &textproto.Error{
		Code: smtpPermanentErr,
		Msg:  "authentication required",
	})

    err := worker.retryOrFail(ctx, pending, "send", smtpErr)
    if err != nil {
        t.Fatalf("expected retryOrFail to succeed, got error: %v", err)
    }

    if !mockRepo.markAsFailedCalled {
        t.Fatal("expected MarkAsFailed to be called")
    }

    if mockRepo.markForRetryCalled {
        t.Fatal("expected MarkForRetry not to be called")
    }

    if mockRepo.id != pending.Id {
        t.Fatalf("expected queue id %d, got %d", pending.Id, mockRepo.id)
    }
}

func TestRetryOrFailSMTPTemporaryError(t *testing.T) {
    ctx := context.Background()

    mockRepo := &MockEmailQueueRepository{}

    worker := &EmailWorker{
        emailQueueRepo: mockRepo,
    }

    pending := EmailQueue{
        Id: 2,
        Attempts: 0,
    }

    smtpErr := fmt.Errorf("error: %w", &textproto.Error{
		Code: smtpTemporaryErr,
		Msg:  "domain not found",
	})

    err := worker.retryOrFail(ctx, pending, "send", smtpErr)
    if err != nil {
        t.Fatalf("expected retryOrFail to succeed, got error: %v", err)
    }

    if !mockRepo.markForRetryCalled {
        t.Fatal("expected MarkForRetry to be called")
    }

    if mockRepo.markAsFailedCalled {
        t.Fatal("expected MarkAsFailed not to be called")
    }

    if mockRepo.id != pending.Id {
        t.Fatalf("expected queue id %d, got %d", pending.Id, mockRepo.id)
    }
}
