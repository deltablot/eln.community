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

// RorOrganization represents a ROR organization from the API
type RorOrganization struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Types []string `json:"types,omitempty"`
}

// RorSearchResponse represents the response from ROR API search
type RorSearchResponse struct {
	NumberOfResults int `json:"number_of_results"`
	Items           []struct {
		ID    string `json:"id"`
		Names []struct {
			Lang  *string  `json:"lang"`
			Types []string `json:"types"`
			Value string   `json:"value"`
		} `json:"names"`
		Types []string `json:"types"`
	} `json:"items"`
}

// RorDetailResponse represents the response from ROR API for a single organization
type RorDetailResponse struct {
	ID    string `json:"id"`
	Names []struct {
		Lang  *string  `json:"lang"`
		Types []string `json:"types"`
		Value string   `json:"value"`
	} `json:"names"`
	Types []string `json:"types"`
}

// RorHandler handles ROR-related HTTP requests
type RorHandler struct {
	httpClient *http.Client
}

// NewRorHandler creates a new ROR handler
func NewRorHandler() *RorHandler {
	return &RorHandler{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SearchRorOrganizations searches for ROR organizations by query
func (h *RorHandler) SearchRorOrganizations(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// Call ROR API
	rorURL := fmt.Sprintf("https://api.ror.org/organizations?query=%s", url.QueryEscape(query))
	resp, err := h.httpClient.Get(rorURL)
	if err != nil {
		log.Printf("Error calling ROR API: %v", err)
		http.Error(w, "Error searching ROR organizations", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ROR API returned status %d", resp.StatusCode)
		http.Error(w, "Error from ROR API", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading ROR API response: %v", err)
		http.Error(w, "Error reading ROR API response", http.StatusInternalServerError)
		return
	}

	var rorResp RorSearchResponse
	if err := json.Unmarshal(body, &rorResp); err != nil {
		log.Printf("Error parsing ROR API response: %v", err)
		http.Error(w, "Error parsing ROR API response", http.StatusInternalServerError)
		return
	}

	// Transform to our simplified format
	organizations := make([]RorOrganization, 0, len(rorResp.Items))
	for _, item := range rorResp.Items {
		// Extract ROR ID from URL (e.g., "https://ror.org/042nb2s44" -> "042nb2s44")
		rorID := extractRorID(item.ID)

		// Find the display name (prefer ror_display type)
		displayName := getDisplayName(item.Names)

		organizations = append(organizations, RorOrganization{
			ID:    rorID,
			Name:  displayName,
			Types: item.Types,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}

// GetRorOrganization gets details for a specific ROR ID
func (h *RorHandler) GetRorOrganization(w http.ResponseWriter, r *http.Request, rorID string) {
	// Validate ROR ID format
	normalizedID, isValid := validateAndNormalizeRorId(rorID)
	if !isValid {
		http.Error(w, "Invalid ROR ID format", http.StatusBadRequest)
		return
	}

	// Call ROR API
	rorURL := fmt.Sprintf("https://api.ror.org/organizations/%s", normalizedID)
	resp, err := h.httpClient.Get(rorURL)
	if err != nil {
		log.Printf("Error calling ROR API: %v", err)
		http.Error(w, "Error fetching ROR organization", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.NotFound(w, r)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("ROR API returned status %d", resp.StatusCode)
		http.Error(w, "Error from ROR API", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading ROR API response: %v", err)
		http.Error(w, "Error reading ROR API response", http.StatusInternalServerError)
		return
	}

	var rorResp RorDetailResponse
	if err := json.Unmarshal(body, &rorResp); err != nil {
		log.Printf("Error parsing ROR API response: %v", err)
		http.Error(w, "Error parsing ROR API response", http.StatusInternalServerError)
		return
	}

	// Transform to our simplified format
	organization := RorOrganization{
		ID:    normalizedID,
		Name:  getDisplayName(rorResp.Names),
		Types: rorResp.Types,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organization)
}

// GetRorOrganizations gets details for multiple ROR IDs
func (h *RorHandler) GetRorOrganizations(w http.ResponseWriter, r *http.Request) {
	rorIDsParam := r.URL.Query().Get("ids")
	if rorIDsParam == "" {
		http.Error(w, "Query parameter 'ids' is required", http.StatusBadRequest)
		return
	}

	rorIDs := strings.Split(rorIDsParam, ",")
	organizations := make([]RorOrganization, 0, len(rorIDs))

	for _, rorID := range rorIDs {
		rorID = strings.TrimSpace(rorID)
		if rorID == "" {
			continue
		}

		// Validate ROR ID format
		normalizedID, isValid := validateAndNormalizeRorId(rorID)
		if !isValid {
			log.Printf("Invalid ROR ID: %s", rorID)
			continue
		}

		// Call ROR API
		rorURL := fmt.Sprintf("https://api.ror.org/organizations/%s", normalizedID)
		resp, err := h.httpClient.Get(rorURL)
		if err != nil {
			log.Printf("Error calling ROR API for %s: %v", normalizedID, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("ROR API returned status %d for %s", resp.StatusCode, normalizedID)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Error reading ROR API response for %s: %v", normalizedID, err)
			continue
		}

		var rorResp RorDetailResponse
		if err := json.Unmarshal(body, &rorResp); err != nil {
			log.Printf("Error parsing ROR API response for %s: %v", normalizedID, err)
			continue
		}

		organizations = append(organizations, RorOrganization{
			ID:    normalizedID,
			Name:  getDisplayName(rorResp.Names),
			Types: rorResp.Types,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}

// Router handles routing for ROR endpoints
func (h *RorHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/ror/search" && r.Method == "GET":
		h.SearchRorOrganizations(w, r)
	case path == "/api/v1/ror/organizations" && r.Method == "GET":
		h.GetRorOrganizations(w, r)
	case strings.HasPrefix(path, "/api/v1/ror/organization/") && r.Method == "GET":
		rorID := strings.TrimPrefix(path, "/api/v1/ror/organization/")
		h.GetRorOrganization(w, r, rorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Helper functions

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
