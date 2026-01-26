package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// EmailService handles sending emails via SMTP2GO
type EmailService struct {
	apiKey   string
	fromAddr string
	fromName string
	enabled  bool
}

// NewEmailService creates a new email service instance
func NewEmailService() *EmailService {
	apiKey := os.Getenv("SMTP2GO_API_KEY")
	fromAddr := os.Getenv("EMAIL_FROM_ADDRESS")
	fromName := os.Getenv("EMAIL_FROM_NAME")

	if fromAddr == "" {
		fromAddr = "noreply@eln.community"
	}
	if fromName == "" {
		fromName = "ELN Community"
	}

	enabled := apiKey != ""
	if !enabled {
		log.Println("Email service disabled: SMTP2GO_API_KEY not configured")
	}

	return &EmailService{
		apiKey:   apiKey,
		fromAddr: fromAddr,
		fromName: fromName,
		enabled:  enabled,
	}
}

// SMTP2GORequest represents the request structure for SMTP2GO API
type SMTP2GORequest struct {
	APIKey        string              `json:"api_key"`
	To            []string            `json:"to"`
	Sender        string              `json:"sender"`
	Subject       string              `json:"subject"`
	TextBody      string              `json:"text_body,omitempty"`
	HTMLBody      string              `json:"html_body,omitempty"`
	CustomHeaders []map[string]string `json:"custom_headers,omitempty"`
}

// SendNewRecordNotification sends an email notification to admins about a new record
func (s *EmailService) SendNewRecordNotification(adminEmails []string, record *Record, siteURL string) error {
	if !s.enabled {
		log.Println("Email service disabled, skipping notification")
		return nil
	}

	if len(adminEmails) == 0 {
		log.Println("No admin emails configured, skipping notification")
		return nil
	}

	subject := fmt.Sprintf("New Record Uploaded: %s", record.Name)

	recordURL := fmt.Sprintf("%s/record/%s", siteURL, record.Id)

	textBody := fmt.Sprintf(`A new record has been uploaded to ELN Community.

Record Details:
- Name: %s
- Uploader: %s (ORCID: %s)
- Uploaded: %s
- ID: %s

View the record: %s

---
This is an automated notification from ELN Community.
`,
		record.Name,
		record.UploaderName,
		record.UploaderOrcid,
		record.CreatedAt.Format("2006-01-02 15:04:05 MST"),
		record.Id,
		recordURL,
	)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; }
        .detail { margin: 10px 0; }
        .label { font-weight: bold; }
        .button { display: inline-block; padding: 10px 20px; background-color: #4CAF50; color: white; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { margin-top: 20px; padding-top: 20px; border-top: 1px solid #ddd; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>New Record Uploaded</h2>
        </div>
        <div class="content">
            <p>A new record has been uploaded to ELN Community.</p>
            
            <div class="detail">
                <span class="label">Record Name:</span> %s
            </div>
            <div class="detail">
                <span class="label">Uploader:</span> %s (ORCID: %s)
            </div>
            <div class="detail">
                <span class="label">Uploaded:</span> %s
            </div>
            <div class="detail">
                <span class="label">Record ID:</span> %s
            </div>
            
            <a href="%s" class="button">View Record</a>
        </div>
        <div class="footer">
            This is an automated notification from ELN Community.
        </div>
    </div>
</body>
</html>`,
		record.Name,
		record.UploaderName,
		record.UploaderOrcid,
		record.CreatedAt.Format("2006-01-02 15:04:05 MST"),
		record.Id,
		recordURL,
	)

	return s.sendEmail(adminEmails, subject, textBody, htmlBody)
}

// sendEmail sends an email via SMTP2GO API
func (s *EmailService) sendEmail(to []string, subject, textBody, htmlBody string) error {
	if !s.enabled {
		return fmt.Errorf("email service not enabled")
	}

	reqBody := SMTP2GORequest{
		APIKey:   s.apiKey,
		To:       to,
		Sender:   fmt.Sprintf("%s <%s>", s.fromName, s.fromAddr),
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.smtp2go.com/v3/email/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		return fmt.Errorf("SMTP2GO API error (status %d): %v", resp.StatusCode, errorResp)
	}

	log.Printf("Email notification sent successfully to %d recipients", len(to))
	return nil
}
