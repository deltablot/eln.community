package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
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
	client *RorClient
}

// NewRorHandler creates a new ROR handler
func NewRorHandler() *RorHandler {
	return &RorHandler{
		client: NewRorClient(),
	}
}

// SearchRorOrganizations searches for ROR organizations by query
func (h *RorHandler) SearchRorOrganizations(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	organizations, err := h.client.SearchOrganizations(query)
	if err != nil {
		log.Printf("Error searching ROR organizations: %v", err)
		http.Error(w, "Error searching ROR organizations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}

// GetRorOrganization gets details for a specific ROR ID
func (h *RorHandler) GetRorOrganization(w http.ResponseWriter, r *http.Request, rorID string) {
	organization, err := h.client.GetOrganization(rorID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(err.Error(), "invalid") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("Error fetching ROR organization: %v", err)
		http.Error(w, "Error fetching ROR organization", http.StatusInternalServerError)
		return
	}

	// Check if it was a cache hit
	cacheHeader := "MISS"
	if _, found := h.client.cache.Get(rorID); found {
		cacheHeader = "HIT"
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", cacheHeader)
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
	organizations := h.client.GetOrganizations(rorIDs)

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
