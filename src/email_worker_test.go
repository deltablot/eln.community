package main

import (
	"context"
	"fmt"
	"net/textproto"
	"testing"
)

const smtpPermanentErr = 530
const smtpTemporaryErr = 450

type MockEmailQueueRepository struct {
	markAsFailedCalled bool
	markForRetryCalled bool
	id                 int64
	lastError          string
	getPendingCalled   bool
	pendingToReturn    []EmailQueue
	markAsSentCalled   bool
}

type MockEmailSender struct {
	sendCalled  bool
	subject     string
	bodyText    string
	bodyHTML    string
	to          string
	errToReturn error
}

type MockOrcidService struct {
	getEmailCalled bool
	orcidReceived  string
	emailToReturn  string
	errToReturn    error
}

func (m *MockEmailQueueRepository) Enqueue(ctx context.Context, item *EmailQueue) (*EmailQueue, error) {
	return nil, nil
}

func (m *MockEmailQueueRepository) GetPending(ctx context.Context, limit int) ([]EmailQueue, error) {
	m.getPendingCalled = true
	return m.pendingToReturn, nil
}

func (m *MockEmailQueueRepository) MarkAsSent(ctx context.Context, id int64) error {
	m.markAsSentCalled = true
	m.id = id
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

func (m *MockEmailSender) Send(ctx context.Context, to string, subject string, bodyText string, bodyHTML string) error {
	m.sendCalled = true
	m.to = to
	m.subject = subject
	m.bodyText = bodyText
	m.bodyHTML = bodyHTML
	return m.errToReturn
}

func (m *MockOrcidService) GetEmail(ctx context.Context, orcid string) (string, error) {
	m.getEmailCalled = true
	m.orcidReceived = orcid
	return m.emailToReturn, m.errToReturn
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
		Id:       2,
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
		t.Fatalf("expected queue id: %d, got: %d", pending.Id, mockRepo.id)
	}
}

func TestRetryOrFailSMTPTemporaryError(t *testing.T) {
	ctx := context.Background()

	mockRepo := &MockEmailQueueRepository{}

	worker := &EmailWorker{
		emailQueueRepo: mockRepo,
	}

	pending := EmailQueue{
		Id:       2,
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
		t.Fatalf("expected queue id: %d, got: %d", pending.Id, mockRepo.id)
	}
}

func TestProcessPendingMarkAsSentWhenSuccess(t *testing.T) {
	ctx := context.Background()

	mockRepo := &MockEmailQueueRepository{}
	mockSender := &MockEmailSender{}
	mockOrcid := &MockOrcidService{}

	worker := &EmailWorker{
		emailQueueRepo: mockRepo,
		emailSender:    mockSender,
		orcidService:   mockOrcid,
	}

	pending := EmailQueue{
		Id:             2,
		RecipientOrcid: "0000-0000-0000-0000",
		Subject:        "Test",
		BodyText:       "Body test",
		BodyHTML:       "<p>Body test</p>",
	}

	mockRepo.pendingToReturn = []EmailQueue{pending}
	mockOrcid.emailToReturn = "test@test.com"

	err := worker.ProcessPending(ctx, 1)
	if err != nil {
		t.Fatalf("expected ProcessPending to succeed, got error: %v", err)
	}

	if !mockRepo.getPendingCalled {
		t.Fatal("expected GetPending to be called")
	}

	if !mockOrcid.getEmailCalled {
		t.Fatal("expected getEmail to be called")
	}

	if mockOrcid.orcidReceived != pending.RecipientOrcid {
		t.Fatalf("expected orcid: %q, got: %q", pending.RecipientOrcid, mockOrcid.orcidReceived)
	}

	if !mockSender.sendCalled {
		t.Fatal("expected Send to be called")
	}

	if mockSender.to != mockOrcid.emailToReturn {
		t.Fatalf("expected email: %q, got: %q", mockOrcid.emailToReturn, mockSender.to)
	}

	if mockSender.subject != pending.Subject {
		t.Fatalf("expected subject: %q, got: %q", pending.Subject, mockSender.subject)
	}

	if mockSender.bodyText != pending.BodyText {
		t.Fatalf("expected bodyText: %q, got: %q", pending.BodyText, mockSender.bodyText)
	}

	if mockSender.bodyHTML != pending.BodyHTML {
		t.Fatalf("expected bodyHTML: %q, got: %q", pending.BodyHTML, mockSender.bodyHTML)
	}

	if !mockRepo.markAsSentCalled {
		t.Fatal("expected MarkAsSent to be called")
	}

	if mockRepo.id != pending.Id {
		t.Fatalf("expected queue id: %d, got: %d", pending.Id, mockRepo.id)
	}

	if mockRepo.markForRetryCalled {
		t.Fatal("expected MarkForRetry not to be called")
	}

	if mockRepo.markAsFailedCalled {
		t.Fatal("expected MarkAsFailed not to be called")
	}
}

func TestProcessPendingMarkAsFailedWhenEmailUnavailable(t *testing.T) {
	ctx := context.Background()

	mockRepo := &MockEmailQueueRepository{}
	mockSender := &MockEmailSender{}
	mockOrcid := &MockOrcidService{}

	worker := &EmailWorker{
		emailQueueRepo: mockRepo,
		emailSender:    mockSender,
		orcidService:   mockOrcid,
	}

	pending := EmailQueue{
		Id:             2,
		RecipientOrcid: "0000-0000-0000-0000",
	}

	mockRepo.pendingToReturn = []EmailQueue{pending}
	mockOrcid.emailToReturn = ""
	mockOrcid.errToReturn = &EmailUnavailable{Orcid: pending.RecipientOrcid}

	err := worker.ProcessPending(ctx, 1)
	if err != nil {
		t.Fatalf("expected ProcessPending to succeed, got error: %v", err)
	}

	if !mockRepo.getPendingCalled {
		t.Fatal("expected GetPending to be called")
	}

	if !mockOrcid.getEmailCalled {
		t.Fatal("expected getEmail to be called")
	}

	if mockOrcid.orcidReceived != pending.RecipientOrcid {
		t.Fatalf("expected orcid: %q, got: %q", pending.RecipientOrcid, mockOrcid.orcidReceived)
	}

	if mockSender.sendCalled {
		t.Fatal("expected Send not to be called")
	}

	if mockRepo.markAsSentCalled {
		t.Fatal("expected MarkAsSent not to be called")
	}

	if mockRepo.markForRetryCalled {
		t.Fatal("expected MarkForRetry not to be called")
	}

	if !mockRepo.markAsFailedCalled {
		t.Fatal("expected MarkAsFailed to be called")
	}

	if mockRepo.id != pending.Id {
		t.Fatalf("expected queue id: %d, got: %d", pending.Id, mockRepo.id)
	}
}

func TestProcessPendingMarkAsFailedWithSMTPPermanentErr(t *testing.T) {
	ctx := context.Background()

	mockRepo := &MockEmailQueueRepository{}
	mockSender := &MockEmailSender{}
	mockOrcid := &MockOrcidService{}

	worker := &EmailWorker{
		emailQueueRepo: mockRepo,
		emailSender:    mockSender,
		orcidService:   mockOrcid,
	}

	pending := EmailQueue{
		Id:             2,
		RecipientOrcid: "0000-0000-0000-0000",
	}

	mockRepo.pendingToReturn = []EmailQueue{pending}
	mockOrcid.emailToReturn = "test@test.com"
	smtpErr := fmt.Errorf("error: %w", &textproto.Error{
		Code: smtpPermanentErr,
		Msg:  "authentication required",
	})
	mockSender.errToReturn = smtpErr

	err := worker.ProcessPending(ctx, 1)
	if err != nil {
		t.Fatalf("expected ProcessPending to succeed, got error: %v", err)
	}

	if !mockRepo.getPendingCalled {
		t.Fatal("expected GetPending to be called")
	}

	if !mockOrcid.getEmailCalled {
		t.Fatal("expected getEmail to be called")
	}

	if mockOrcid.orcidReceived != pending.RecipientOrcid {
		t.Fatalf("expected orcid: %q, got: %q", pending.RecipientOrcid, mockOrcid.orcidReceived)
	}

	if !mockSender.sendCalled {
		t.Fatal("expected Send to be called")
	}

	if mockSender.to != mockOrcid.emailToReturn {
		t.Fatalf("expected email: %q, got: %q", mockOrcid.emailToReturn, mockSender.to)
	}

	if mockRepo.markAsSentCalled {
		t.Fatal("expected MarkAsSent not to be called")
	}

	if mockRepo.markForRetryCalled {
		t.Fatal("expected MarkForRetry not to be called")
	}

	if !mockRepo.markAsFailedCalled {
		t.Fatal("expected MarkAsFailed to be called")
	}

	if mockRepo.id != pending.Id {
		t.Fatalf("expected queue id: %d, got: %d", pending.Id, mockRepo.id)
	}
}
