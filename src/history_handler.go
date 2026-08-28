package main

import (
	"net/http"
	"strings"
	"time"
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

const historyHandlerErr = "history handler:"

// VersionSummary is a lightweight version info for dropdown
type VersionSummary struct {
	Version          int              `json:"version"`
	Name             string           `json:"name"`
	ArchivedAt       time.Time        `json:"archived_at"`
	ModerationStatus ModerationStatus `json:"moderation_status"`
}

// Router handles routing for history endpoints
// GET /api/v1/records/{id}/versions - Get list of versions (lightweight)
func (h *HistoryHandler) Router(w http.ResponseWriter, r *http.Request) {
	res := APIResponse[VersionSummary]{}
	const prefix = "/api/v1/records/"

	res.Data = []VersionSummary{}
	if r.Method != "GET" {
		res.Meta.Error.Code = http.StatusMethodNotAllowed
		res.Meta.Error.Message = http.StatusText(res.Meta.Error.Code)
		res.Meta.Error.Description = "the requested HTTP method is not supported for this endpoint"
		errorLogger.Printf("%s: failed to get record version", historyHandlerErr)
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
	status := http.StatusNotFound
	res.Meta.Error.Code = status
	res.Meta.Error.Message = http.StatusText(status)
	res.Meta.Error.Description = "record version not found"
	errorLogger.Printf("%s: record version not found", historyHandlerErr)
	writeJson(w, status, res)

}

// GetVersionsList handles GET /api/v1/records/{id}/versions
// Returns lightweight list of version numbers and timestamps
// Only shows approved versions to regular users
func (h *HistoryHandler) GetVersionsList(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	res := APIResponse[VersionSummary]{}

	res.Data = []VersionSummary{}
	if !uuidv7Regex.MatchString(id) {
		status := http.StatusBadRequest
		res.Meta.Error.Code = status
		res.Meta.Error.Message = http.StatusText(status)
		res.Meta.Error.Description = "invalid record version ID format"
		errorLogger.Printf("%s: invalid record version ID format", historyHandlerErr)
		writeJson(w, status, res)
		return
	}

	// Get the record to check ownership
	record, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		status := http.StatusNotFound
		res.Meta.Error.Code = status
		res.Meta.Error.Message = http.StatusText(status)
		res.Meta.Error.Description = "record version not found"
		errorLogger.Printf("%s: record version not found", historyHandlerErr)
		writeJson(w, status, res)
		return
	}

	user, isAuthenticated := userFromSession(ctx)

	isAdmin, err := currentUserIsAdmin(w, r, h.adminRepo)
	if err != nil {
		return
	}
	isOwner := isAuthenticated && record.UploaderOrcid == user.Orcid

	// Get history
	history, err := h.historyRepo.GetHistory(ctx, id)
	if err != nil {
		res.Meta.Error.Code = http.StatusInternalServerError
		res.Meta.Error.Message = http.StatusText(res.Meta.Error.Code)
		res.Meta.Error.Description = "database error"
		errorLogger.Printf("%s: failed to get record version: %v", recordHandlerErr, err)
		return
	}

	// Filter versions based on user permissions
	var filteredHistory []RecordsRevisions
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
			ArchivedAt:       h.ArchivedAt.UTC(),
			ModerationStatus: h.ModerationStatus,
		}
	}

	res.Data = versions
	writeJson(w, http.StatusOK, res)
}
