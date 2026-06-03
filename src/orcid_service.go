package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type OrcidService struct {
}

type OrcidResolver interface {
	GetEmailFromOrcid(ctx context.Context, orcid string) (string, error)
}

func NewOrcidService() *OrcidService {
	return &OrcidService{}
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
		return "", fmt.Errorf("Orcid Service: failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to send token request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Orcid Service: token request failed with status %d", res.StatusCode)
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(res.Body).Decode(&token)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to decode token response: %w", err)
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("Orcid Service: token response did not contain access_token")
	}

	return token.AccessToken, nil
}

func (o *OrcidService) GetEmailFromOrcid(ctx context.Context, orcid string) (string, error) {
	token, err := getOrcidAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: fail to get access token: %v", err)
	}

	address := strings.Join([]string{"https://pub.orcid.org/v3.0/", orcid, "/email"}, "")

	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return "", fmt.Errorf("Orcid Service: failed to create email request for orcid %q: %w", orcid, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := http.DefaultClient.Do(req)
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
