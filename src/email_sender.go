package main

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"os"
	"strings"
)

type EmailSender struct {
	smtpHost        string
	smtpPort        string
	smtpFromAddress string
	smtpUsername    string
	smtpPassword    string
}

func NewEmailSender() *EmailSender {
	return &EmailSender{
		smtpHost:        os.Getenv("SMTP_HOST"),
		smtpPort:        os.Getenv("SMTP_PORT"),
		smtpFromAddress: os.Getenv("SMTP_FROM_ADDRESS"),
		smtpUsername:    os.Getenv("SMTP_USERNAME"),
		smtpPassword:    os.Getenv("SMTP_PASSWORD"),
	}
}

func (e *EmailSender) Send(to string, subject string, bodyText string, bodyHTML string) error {
	// Sanitize 'to' to prevent header injection
	if strings.ContainsAny(to, "\r\n") {
		return fmt.Errorf("email sender: recipient address contains invalid CRLF characters")
	}
	var smtpAddr = net.JoinHostPort(e.smtpHost, e.smtpPort)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	boundary := w.Boundary()

	auth := smtp.PlainAuth("", e.smtpUsername, e.smtpPassword, e.smtpHost)
	recipients := []string{to}
	msg := []byte(
		"From: " + e.smtpFromAddress + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n" +
			"\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			bodyText + "\r\n" +
			"\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			bodyHTML + "\r\n" +
			"\r\n" +
			"--" + boundary + "--\r\n",
	)

	err := smtp.SendMail(smtpAddr, auth, e.smtpFromAddress, recipients, msg)
	if err != nil {
		return fmt.Errorf("email sender: failed to send email: %w", err)
	}
	return nil
}
