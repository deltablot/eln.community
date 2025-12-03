package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// HistoryHandler handles version history endpoints
type HistoryHandler struct {
	historyRepo HistoryRepository
	recordRepo  RecordRepository
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(historyRepo HistoryRepository, recordRepo RecordRepository) *HistoryHandler {
	return &HistoryHandler{
		historyRepo: historyRepo,
		recordRepo:  recordRepo,
	}
}

// Router handles routing for history endpoints
// GET /api/v1/records/{id}/history - Get all versions
// GET /api/v1/records/{id}/history/{version} - Get specific version
func (h *HistoryHandler) Router(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/records/"

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	// Expected: {id}/history or {id}/history/{version}
	parts := strings.Split(raw, "/")

	// parts[0] = id, parts[1] = "history", parts[2] = version (optional)
	if len(parts) >= 2 && parts[1] == "history" {
		id := parts[0]

		if len(parts) == 2 {
			// GET /api/v1/records/{id}/history
			h.GetHistory(w, r, id)
			return
		}

		if len(parts) == 3 && parts[2] != "" {
			// GET /api/v1/records/{id}/history/{version}
			h.GetVersion(w, r, id, parts[2])
			return
		}
	}

	http.NotFound(w, r)
}

// GetHistory handles GET /api/v1/records/{id}/history
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request, id string) {
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	// Get current record first
	currentRecord, err := h.recordRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Get history
	history, err := h.historyRepo.GetHistory(r.Context(), id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Build response with current version + history
	response := struct {
		Current *Record         `json:"current"`
		History []RecordHistory `json:"history"`
		Total   int             `json:"total"`
	}{
		Current: currentRecord,
		History: history,
		Total:   len(history) + 1, // +1 for current
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetVersion handles GET /api/v1/records/{id}/history/{version}
func (h *HistoryHandler) GetVersion(w http.ResponseWriter, r *http.Request, id string, versionStr string) {
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	// Get specific version from history
	historyRecord, err := h.historyRepo.GetVersion(r.Context(), id, version)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(historyRecord)
}
