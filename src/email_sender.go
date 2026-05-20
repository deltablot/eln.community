package main

import (
	"log"
	"net/smtp"
	"os"
)

func SendEmail() {
	var smtp_host = "mail.smtp2go.com"
	var app_mail = os.Getenv("EMAIL_FROM_ADDRESS")
	var admin_mail = os.Getenv("ADMIN_EMAIL")
	var smtpUsername = os.Getenv("SMTP_USERNAME")
	var smtpPassword = os.Getenv("SMTP_PASSWORD")
	var port = ":2525"
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtp_host)

	to := []string{admin_mail}
	msg := []byte(
		"From: " + app_mail + "\r\n" +
			"To: " + admin_mail + "\r\n" +
			"Subject: Test SMTP email\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			"This is the email body.\r\n",
	)
	err := smtp.SendMail(smtp_host+port, auth, app_mail, to, msg)
	if err != nil {
		log.Fatal(err)
	}
}
