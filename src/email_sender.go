package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
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

const smtpTimeout = 60 * time.Second
const dialTimeout = 10 * time.Second

func emailSenderErr(msg string, err error) error {
	return fmt.Errorf("email sender: failed to %s: %w", msg, err)
}

func (e *EmailSender) Send(to string, subject string, bodyText string, bodyHTML string) error {
	// Sanitize 'to' to prevent header injection
	if strings.ContainsAny(to, "\r\n") {
        return fmt.Errorf("email sender: failed to validate recipient address: contains CRLF characters")
	}
	smtpAddr := net.JoinHostPort(e.smtpHost, e.smtpPort)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	boundary := w.Boundary()

	msg := []byte(
		"From: " + e.smtpFromAddress + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n" +
			"\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
           	"Content-Transfer-Encoding: 8bit\r\n" +
			"\r\n" +
			bodyText + "\r\n" +
			"\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
           	"Content-Transfer-Encoding: 8bit\r\n" +
			"\r\n" +
			bodyHTML + "\r\n" +
			"\r\n" +
			"--" + boundary + "--\r\n",
	)

	deadline := time.Now().Add(smtpTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), smtpTimeout)
	defer cancel()

	dialer := net.Dialer{
		Timeout: dialTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", smtpAddr)
	if err != nil {
		return emailSenderErr("dial SMTP server", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return emailSenderErr("set SMTP connection deadline", err)
	}

	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return emailSenderErr("create SMTP client", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: e.smtpHost,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return emailSenderErr("start TLS", err)
	}

	auth := smtp.PlainAuth("", e.smtpUsername, e.smtpPassword, e.smtpHost)
	if err := client.Auth(auth); err != nil {
		return emailSenderErr("authenticate with SMTP server", err)
	}

	if err := client.Mail(e.smtpFromAddress); err != nil {
		return emailSenderErr("set SMTP sender", err)
	}

	if err := client.Rcpt(to); err != nil {
		return emailSenderErr("set SMTP recipient", err)
	}

	writer, err := client.Data()
	if err != nil {
		return emailSenderErr("open SMTP data writer", err)
	}
	if _, err := writer.Write(msg); err != nil {
		return emailSenderErr("write SMTP message", err)
	}
	if err := writer.Close(); err != nil {
		return emailSenderErr("close SMTP data writer", err)
	}

	if err := client.Quit(); err != nil {
		return emailSenderErr("quit SMTP session", err)
	}

	return nil
}
