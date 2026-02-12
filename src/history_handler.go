package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HistoryHandler handles version history endpoints
type HistoryHandler struct {
	historyRepo HistoryRepository
	recordRepo  RecordRepository
	adminRepo   AdminRepository
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(historyRepo HistoryRepository, recordRepo RecordRepository, adminRepo AdminRepository) *HistoryHandler {
	return &HistoryHandler{
		historyRepo: historyRepo,
		recordRepo:  recordRepo,
		adminRepo:   adminRepo,
	}
}

// VersionSummary is a lightweight version info for dropdown
type VersionSummary struct {
	Version          int              `json:"version"`
	Name             string           `json:"name"`
	ArchivedAt       string           `json:"archived_at"`
	ModerationStatus ModerationStatus `json:"moderation_status"`
}

// Router handles routing for history endpoints
// GET /api/v1/records/{id}/versions - Get list of versions (lightweight)
func (h *HistoryHandler) Router(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/records/"

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	// Expected: {id}/versions
	parts := strings.Split(raw, "/")

	if len(parts) == 2 && parts[1] == "versions" {
		// GET /api/v1/records/{id}/versions
		h.GetVersionsList(w, r, parts[0])
		return
	}

	http.NotFound(w, r)
}

// GetVersionsList handles GET /api/v1/records/{id}/versions
// Returns lightweight list of version numbers and timestamps
// Only shows approved versions to regular users
func (h *HistoryHandler) GetVersionsList(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	// Get the record to check ownership
	record, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	// Check if user is authenticated
	orcid, _ := sessionManager.Get(ctx, "orcid").(string)

	// Check if user is admin or owner
	isAdmin := false
	isOwner := false
	if orcid != "" {
		isAdmin, _ = h.adminRepo.IsAdmin(ctx, orcid)
		isOwner = record.UploaderOrcid == orcid
	}

	// Get history
	history, err := h.historyRepo.GetHistory(ctx, id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Filter versions based on user permissions
	var filteredHistory []RecordHistory
	for _, h := range history {
		// Show all versions to admin/owner, only approved to others
		if isAdmin || isOwner || h.ModerationStatus == StatusApproved {
			filteredHistory = append(filteredHistory, h)
		}
	}

	// Build lightweight response
	versions := make([]VersionSummary, len(filteredHistory))
	for i, h := range filteredHistory {
		versions[i] = VersionSummary{
			Version:          h.Version,
			Name:             h.Name,
			ArchivedAt:       h.ArchivedAt.Format("2006-01-02 15:04:05"),
			ModerationStatus: h.ModerationStatus,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
		"total":    len(versions),
	})
}
