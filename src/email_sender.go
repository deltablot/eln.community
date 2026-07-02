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

func requiredEnv(envVar string) (string, error) {
	value := os.Getenv(envVar)
	if value == "" {
		return "", fmt.Errorf("missing required environment variables: %q", envVar)
	}
	return value, nil
}

func NewEmailSender() (*EmailSender, error) {
	smtpHost, err := requiredEnv("SMTP_HOST")
	if err != nil {
		return nil, err
	}
	smtpPort, err := requiredEnv("SMTP_PORT")
	if err != nil {
		return nil, err
	}
	smtpFromAddress, err := requiredEnv("SMTP_FROM_ADDRESS")
	if err != nil {
		return nil, err
	}
	smtpUsername, err := requiredEnv("SMTP_USERNAME")
	if err != nil {
		return nil, err
	}
	smtpPassword, err := requiredEnv("SMTP_PASSWORD")
	if err != nil {
		return nil, err
	}
	return &EmailSender{
		smtpHost:        smtpHost,
		smtpPort:        smtpPort,
		smtpFromAddress: smtpFromAddress,
		smtpUsername:    smtpUsername,
		smtpPassword:    smtpPassword,
	}, nil
}

const smtpTimeout = 60 * time.Second
const dialTimeout = 10 * time.Second

func emailSenderErr(msg string, err error) error {
	return fmt.Errorf("Error: email sender: failed to %s: %w", msg, err)
}

func (e *EmailSender) Send(ctx context.Context, to string, subject string, bodyText string, bodyHTML string) error {
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

	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
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

	// DATA already succeeded; QUIT failure should not trigger a resend path
	_ = client.Quit()

	return nil
}
