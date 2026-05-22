package main

import (
	"log"
    "net"
	"net/smtp"
	"os"
)

func SendEmail() {
	var smtpHost = os.Getenv("SMTP_HOST")
	var appMail = os.Getenv("SMTP_FROM_ADDRESS")
	var smtpUsername = os.Getenv("SMTP_USERNAME")
	var smtpPassword = os.Getenv("SMTP_PASSWORD")
	var smtpPort = os.Getenv("SMTP_PORT")
	var adminMail = os.Getenv("ADMIN_EMAIL")
    var smtpAddr = net.JoinHostPort(smtpHost, smtpPort)

	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	to := []string{adminMail}
	msg := []byte(
		"From: " + appMail + "\r\n" +
			"To: " + adminMail + "\r\n" +
			"Subject: Test SMTP email\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			"This is the email body.\r\n",
	)

	err := smtp.SendMail(smtpAddr, auth, appMail, to, msg)
	if err != nil {
		log.Fatal(err)
	}
}
