package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RorClient handles all interactions with the ROR API
type RorClient struct {
	httpClient *http.Client
	cache      *InMemoryCache[RorOrganization]
	baseURL    string
}

// NewRorClient creates a new ROR API client with caching
func NewRorClient() *RorClient {
	return &RorClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache:   NewInMemoryCache[RorOrganization](24 * time.Hour),
		baseURL: "https://api.ror.org",
	}
}

// NewRorClientWithCache creates a new ROR API client with a custom cache
func NewRorClientWithCache(cache *InMemoryCache[RorOrganization]) *RorClient {
	return &RorClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache:   cache,
		baseURL: "https://api.ror.org",
	}
}

// SearchOrganizations searches for ROR organizations by query string
func (c *RorClient) SearchOrganizations(query string) ([]RorOrganization, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	rorURL := fmt.Sprintf("%s/organizations?query=%s", c.baseURL, url.QueryEscape(query))
	resp, err := c.httpClient.Get(rorURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ROR API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rorResp RorSearchResponse
	if err := json.Unmarshal(body, &rorResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Transform to our simplified format
	organizations := make([]RorOrganization, 0, len(rorResp.Items))
	for _, item := range rorResp.Items {
		rorID := extractRorID(item.ID)
		displayName := getDisplayName(item.Names)

		organizations = append(organizations, RorOrganization{
			ID:    rorID,
			Name:  displayName,
			Types: item.Types,
		})
	}

	return organizations, nil
}

// GetOrganization fetches a single ROR organization by ID (with caching)
func (c *RorClient) GetOrganization(rorID string) (RorOrganization, error) {
	// Validate ROR ID format
	normalizedID, isValid := validateAndNormalizeRorId(rorID)
	if !isValid {
		return RorOrganization{}, fmt.Errorf("invalid ROR ID format: %s", rorID)
	}

	// Check cache first
	if cachedOrg, found := c.cache.Get(normalizedID); found {
		return cachedOrg, nil
	}

	// Fetch from API
	org, err := c.fetchOrganization(normalizedID)
	if err != nil {
		return RorOrganization{}, err
	}

	// Cache the result
	c.cache.Set(normalizedID, org)

	return org, nil
}

// GetOrganizations fetches multiple ROR organizations by IDs (with caching and concurrent fetching)
func (c *RorClient) GetOrganizations(rorIDs []string) []RorOrganization {
	// Normalize and validate all IDs first
	normalizedIDs := make([]string, 0, len(rorIDs))
	for _, rorID := range rorIDs {
		rorID = strings.TrimSpace(rorID)
		if rorID == "" {
			continue
		}

		normalizedID, isValid := validateAndNormalizeRorId(rorID)
		if !isValid {
			log.Printf("Invalid ROR ID: %s", rorID)
			continue
		}
		normalizedIDs = append(normalizedIDs, normalizedID)
	}

	// Check cache for all IDs at once
	cachedOrgs, missingIDs := c.cache.GetMultiple(normalizedIDs)

	organizations := make([]RorOrganization, 0, len(normalizedIDs))

	// Add cached organizations to result
	for _, org := range cachedOrgs {
		organizations = append(organizations, org)
	}

	// Fetch missing IDs from API concurrently
	if len(missingIDs) > 0 {
		log.Printf("Fetching %d ROR organizations concurrently from API", len(missingIDs))
		fetchedOrgs := c.fetchOrganizationsConcurrently(missingIDs)
		organizations = append(organizations, fetchedOrgs...)
		log.Printf("Successfully fetched %d/%d ROR organizations", len(fetchedOrgs), len(missingIDs))
	}

	return organizations
}

// fetchOrganization fetches a single organization from the ROR API (no caching)
func (c *RorClient) fetchOrganization(rorID string) (RorOrganization, error) {
	rorURL := fmt.Sprintf("%s/organizations/%s", c.baseURL, rorID)
	resp, err := c.httpClient.Get(rorURL)
	if err != nil {
		return RorOrganization{}, fmt.Errorf("HTTP request failed for %s: %w", rorID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return RorOrganization{}, fmt.Errorf("organization not found: %s", rorID)
	}

	if resp.StatusCode != http.StatusOK {
		return RorOrganization{}, fmt.Errorf("ROR API returned status %d for %s", resp.StatusCode, rorID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RorOrganization{}, fmt.Errorf("failed to read response for %s: %w", rorID, err)
	}

	var rorResp RorDetailResponse
	if err := json.Unmarshal(body, &rorResp); err != nil {
		return RorOrganization{}, fmt.Errorf("failed to parse JSON for %s: %w", rorID, err)
	}

	org := RorOrganization{
		ID:    rorID,
		Name:  getDisplayName(rorResp.Names),
		Types: rorResp.Types,
	}

	return org, nil
}

// fetchOrganizationsConcurrently fetches multiple ROR organizations concurrently
func (c *RorClient) fetchOrganizationsConcurrently(rorIDs []string) []RorOrganization {
	type result struct {
		org RorOrganization
		err error
	}

	results := make(chan result, len(rorIDs))

	// Fetch each ROR ID concurrently
	for _, rorID := range rorIDs {
		go func(id string) {
			org, err := c.fetchOrganization(id)
			results <- result{org: org, err: err}
		}(rorID)
	}

	// Collect results
	organizations := make([]RorOrganization, 0, len(rorIDs))
	for i := 0; i < len(rorIDs); i++ {
		res := <-results
		if res.err != nil {
			log.Printf("Error fetching ROR organization: %v", res.err)
			continue
		}

		// Cache the result
		c.cache.Set(res.org.ID, res.org)
		organizations = append(organizations, res.org)
	}

	return organizations
}

// GetCacheStats returns statistics about the cache
func (c *RorClient) GetCacheStats() struct {
	Size         int
	ExpiredCount int
	OldestEntry  time.Time
	NewestEntry  time.Time
} {
	return c.cache.GetStats()
}

// ClearCache clears all cached organizations
func (c *RorClient) ClearCache() {
	c.cache.Clear()
}

// extractRorID extracts the ROR ID from a full ROR URL
func extractRorID(rorURL string) string {
	// Extract ID from "https://ror.org/042nb2s44"
	parts := strings.Split(rorURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return rorURL
}

// getDisplayName finds the best display name from the names array
func getDisplayName(names []struct {
	Lang  *string  `json:"lang"`
	Types []string `json:"types"`
	Value string   `json:"value"`
}) string {
	// Priority: ror_display > label > first name
	var labelName, firstName string

	for _, name := range names {
		if firstName == "" {
			firstName = name.Value
		}

		for _, t := range name.Types {
			if t == "ror_display" {
				return name.Value
			}
			if t == "label" && labelName == "" {
				labelName = name.Value
			}
		}
	}

	if labelName != "" {
		return labelName
	}
	return firstName
}
