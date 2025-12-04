package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HistoryHandler handles version history endpoints
type HistoryHandler struct {
	historyRepo HistoryRepository
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(historyRepo HistoryRepository) *HistoryHandler {
	return &HistoryHandler{
		historyRepo: historyRepo,
	}
}

// VersionSummary is a lightweight version info for dropdown
type VersionSummary struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	ArchivedAt string `json:"archived_at"`
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
func (h *HistoryHandler) GetVersionsList(w http.ResponseWriter, r *http.Request, id string) {
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	// Get history
	history, err := h.historyRepo.GetHistory(r.Context(), id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Build lightweight response
	versions := make([]VersionSummary, len(history))
	for i, h := range history {
		versions[i] = VersionSummary{
			Version:    h.Version,
			Name:       h.Name,
			ArchivedAt: h.ArchivedAt.Format("2006-01-02 15:04:05"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
		"total":    len(versions),
	})
}
