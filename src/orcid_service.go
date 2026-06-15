package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OrcidService struct {
}

type OrcidResolver interface {
	GetEmail(ctx context.Context, orcid string) (string, error)
}

func NewOrcidService() *OrcidService {
	return &OrcidService{}
}

var orcidHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// https://info.orcid.org/documentation/api-tutorials/api-tutorial-read-data-on-a-record
func (o *OrcidService) GetEmail(ctx context.Context, orcid string) (string, error) {
	address := strings.Join([]string{"https://pub.orcid.org/v3.0/", orcid, "/email"}, "")

	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to create email request for orcid %q: %w", orcid, err)
	}

	req.Header.Set("Accept", "application/json")

	//res, err := http.DefaultClient.Do(req)
	res, err := orcidHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to send email request for orcid %q: %w", orcid, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Orcid Service: email request for orcid %q failed with status: %d", orcid, res.StatusCode)
	}

	var apiResponse struct {
		Emails []struct {
			Email      string `json:"email"`
			Visibility string `json:"visibility"`
			Verified   bool   `json:"verified"`
			Primary    bool   `json:"primary"`
		} `json:"email"`
	}
	err = json.NewDecoder(res.Body).Decode(&apiResponse)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to decode email response for orcid %q: %w", orcid, err)
	}

	if len(apiResponse.Emails) == 0 {
		return "", fmt.Errorf("Orcid Service error: no email return for orcid %q", orcid)
	}

	for _, email := range apiResponse.Emails {
		if email.Email != "" {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("Orcid Service: email is not available for orcid: %s", orcid)
}
