package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type ModerationHandler struct {
	moderationRepo ModerationRepository
	adminRepo      AdminRepository
}

func NewModerationHandler(moderationRepo ModerationRepository, adminRepo AdminRepository) *ModerationHandler {
	return &ModerationHandler{
		moderationRepo: moderationRepo,
		adminRepo:      adminRepo,
	}
}

// GetModerationQueue handles GET /moderation - Admin page to review pending records
func (h *ModerationHandler) GetModerationQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated and is admin
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Parse pagination
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20
	offset := (page - 1) * pageSize

	// Get pending items (both new entries and pending versions)
	items, totalCount, err := h.moderationRepo.GetPendingItems(ctx, pageSize, offset)
	if err != nil {
		http.Error(w, "Error fetching pending items", http.StatusInternalServerError)
		return
	}

	// Prettify metadata for each item
	for i := range items {
		items[i].MetadataPretty = prettyJSON(items[i].Metadata)
	}

	// Get recent moderation history
	var history []ModerationHistoryEntry
	if repo, ok := h.moderationRepo.(*PostgresModerationRepository); ok {
		history, err = repo.GetRecentModerationHistory(ctx, 50)
		if err != nil {
			errorLogger.Printf("Error fetching moderation history: %v", err)
			// Don't fail the request, just show empty history
		}
	}

	name, _ := sessionManager.Get(ctx, "name").(string)
	user := &User{
		Name:  name,
		Orcid: orcid,
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	var pageTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(staticFiles,
		"templates/layout.html",
		"templates/moderation.html",
	))

	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	data := struct {
		App         App
		User        *User
		Items       []PendingItem
		History     []ModerationHistoryEntry
		CurrentPage string
		Page        int
		TotalPages  int
		TotalCount  int
	}{
		App:         app,
		User:        user,
		Items:       items,
		History:     history,
		CurrentPage: "moderation",
		Page:        page,
		TotalPages:  totalPages,
		TotalCount:  totalCount,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// ModerateRecord handles POST /api/v1/moderation/{id} - Approve/reject/flag a record
func (h *ModerationHandler) ModerateRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated and is admin
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Extract record ID from path
	const prefix = "/api/v1/moderation/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, prefix)
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Action string `json:"action"` // "approve", "reject", "flag"
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate action
	var newStatus ModerationStatus
	switch req.Action {
	case "approve":
		newStatus = StatusApproved
		// Check if there's a pending version to approve
		if err := h.moderationRepo.ApprovePendingVersion(ctx, id); err != nil {
			http.Error(w, "Error approving record/version", http.StatusInternalServerError)
			return
		}
	case "reject":
		newStatus = StatusRejected
		// Check if there's a pending version to reject
		if err := h.moderationRepo.RejectPendingVersion(ctx, id); err != nil {
			http.Error(w, "Error rejecting record/version", http.StatusInternalServerError)
			return
		}
	case "flag":
		newStatus = StatusFlagged
		// Update record status
		if err := h.moderationRepo.SetRecordStatus(ctx, id, newStatus); err != nil {
			http.Error(w, "Error updating record status", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid action. Must be 'approve', 'reject', or 'flag'", http.StatusBadRequest)
		return
	}

	// Log moderation action
	action := ModerationAction{
		RecordID:   id,
		AdminOrcid: orcid,
		Action:     req.Action,
		Reason:     req.Reason,
	}
	if err := h.moderationRepo.LogModerationAction(ctx, action); err != nil {
		errorLogger.Printf("Error logging moderation action: %v", err)
		// Don't fail the request if logging fails
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  string(newStatus),
		"message": "Record moderation status updated successfully",
	})
}

// Router handles routing for moderation endpoints
func (h *ModerationHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/moderation" && r.Method == "GET":
		h.GetModerationQueue(w, r)
	case strings.HasPrefix(path, "/api/v1/moderation/") && r.Method == "POST":
		h.ModerateRecord(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
