package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type EmailWorker struct {
	adminRepo      AdminRepository
	emailQueueRepo EmailQueueRepository
	emailSender    *EmailSender
}

// supprimer adminRepo ?
func NewEmailWorker(adminRepo AdminRepository, emailQueueRepo EmailQueueRepository, emailSender *EmailSender) *EmailWorker {
	return &EmailWorker{
		adminRepo:      adminRepo,
		emailQueueRepo: emailQueueRepo,
		emailSender:    emailSender,
	}
}

// https://info.orcid.org/documentation/api-tutorials/api-tutorial-read-data-on-a-record
func getOrcidAccessToken(ctx context.Context) (string, error) {
	data := url.Values{}
	data.Set("client_id", os.Getenv("ORCID_CLIENT_ID"))
	data.Set("client_secret", os.Getenv("ORCID_CLIENT_SECRET"))
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "/read-public")
	encodedData := data.Encode()
	body := strings.NewReader(encodedData)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://orcid.org/oauth/token", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Orcid token request failed with status %d", res.StatusCode)
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(res.Body).Decode(&token)
	if err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("ORCID response did not contain access_token")
	}

	return token.AccessToken, nil
}

// TODO: later, move this function to a new file email_resolver.go
// email_worker should only process queud emails.
//func getEmailFromOrcid(ctx context.Context, orcid string) (string, error) {

//}

func (w *EmailWorker) ProcessPendingEmails(ctx context.Context, limit int) error {
	pendingEmails, err := w.emailQueueRepo.GetPendingEmails(ctx, limit)
	if err != nil {
		log.Printf("failed to get pending emails: %v", err)
		return err
	}
// 	log.Printf("pendingEmails : %v", pendingEmails)

	for _, pending := range pendingEmails {
		getOrcidAccessToken(ctx)
		log.Printf("For=%d", pending.Id)
		/*
		        recipientEmail, err := getEmailFromOrcid(ctx, pending.RecipientOrcid)
			    if err != nil {
		           markErr := w.emailQueueRepo.MarkEmailAsFailed(ctx, pending.Id, err.Error())
		           if markErr != nil {
			  	     log.Printf("failed to get recipient email: %v", markErr)
		             return markErr
		           }
		          continue
		      	}
		        err = w.emailSender.Send(recipientEmail, pending.Subject, pending.Body)
		        if err != nil {
		           markErr := w.emailQueueRepo.MarkEmailAsFailed(ctx, pending.Id, err.Error())
		           if markErr != nil {
                       log.Printf("send failed for email:%d, error: %v", pending.Id, markErr)
		             return markErr
		           }
		           continue
		        }
		        markErr := w.emailQueueRepo.MarkEmailAsSent(ctx, pending.Id)
		        if markErr != nil {
                    log.Printf("For email: %d, error: %v", pending.Id, markErr)
		           return markErr
		        }
                log.Printf("Email sent successfully for email: %d to: %q", pending.Id, recipientEmail)
		*/
	}
	return nil
}
