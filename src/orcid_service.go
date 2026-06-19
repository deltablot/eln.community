package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OrcidClient struct{}

type HTTPStatusError struct {
	StatusCode int
	Message    string
}

type EmailUnavailable struct {
	Orcid   string
	Message string
}

func NewOrcidClient() *OrcidClient {
	return &OrcidClient{}
}

const orcidService = "orcid service"

var orcidHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("%s: %d: failed to %s", orcidService, e.StatusCode, e.Message)
}

func (e *EmailUnavailable) Error() string {
	return fmt.Sprintf("%s: %s for orcid %q", orcidService, e.Message, e.Orcid)
}

// https://info.orcid.org/documentation/api-tutorials/api-tutorial-read-data-on-a-record
func (c *OrcidClient) GetEmail(ctx context.Context, orcid string) (string, error) {
	if strings.TrimSpace(orcid) == "" {
		return "", fmt.Errorf("%s: orcid parameter is empty", orcidService)
	}
	address := strings.Join([]string{"https://pub.orcid.org/v3.0/", orcid, "/email"}, "")

	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return "", fmt.Errorf("%s: failed to create email request for orcid %q: %w", orcidService, orcid, err)
	}

	req.Header.Set("Accept", "application/json")

	res, err := orcidHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: failed to send email request for orcid %q: %w", orcidService, orcid, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", &HTTPStatusError{
			StatusCode: res.StatusCode,
			Message:    fmt.Sprintf("fetch email for orcid %s", orcid),
		}
	}

	var apiResponse struct {
		Emails []struct {
			Email string `json:"email"`
		} `json:"email"`
	}
	err = json.NewDecoder(res.Body).Decode(&apiResponse)
	if err != nil {
		return "", fmt.Errorf("%s: failed to decode email response for orcid %q: %w", orcidService, orcid, err)
	}

	if len(apiResponse.Emails) == 0 {
		return "", &EmailUnavailable{Orcid: orcid, Message: "no email return"}
	}

	for _, email := range apiResponse.Emails {
		if email.Email != "" {
			return email.Email, nil
		}
	}

	return "", &EmailUnavailable{Orcid: orcid, Message: "email is not available"}
}
