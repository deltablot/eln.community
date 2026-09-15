package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type ModerationHandler struct {
	moderationRepo      ModerationRepository
	adminRepo           AdminRepository
	notificationService *NotificationService
	recordRepo          RecordRepository
}

func NewModerationHandler(moderationRepo ModerationRepository, adminRepo AdminRepository, notificationService *NotificationService, recordRepo RecordRepository) *ModerationHandler {
	return &ModerationHandler{
		moderationRepo:      moderationRepo,
		adminRepo:           adminRepo,
		notificationService: notificationService,
		recordRepo:          recordRepo,
	}
}

// GetModerationQueue handles GET /moderation - Admin page to review pending records
func (h *ModerationHandler) GetModerationQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	admin, err := requireAdminUser(w, r, h.adminRepo)
	if err != nil {
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
		errorLogger.Printf("Error: %s", err)
		http.Error(w, "Error fetching pending items", http.StatusInternalServerError)
		return
	}

	// Prettify metadata for each item
	for i := range items {
		items[i].MetadataPretty = prettyJSON(items[i].Metadata)
	}

	// Get recent moderation history
	var history []RecordsModerationLogsEntry
	if repo, ok := h.moderationRepo.(*PostgresModerationRepository); ok {
		history, err = repo.GetRecentRecordsModerationLogs(ctx, 50)
		if err != nil {
			errorLogger.Printf("Error fetching moderation history: %v", err)
			// Don't fail the request, just show empty history
		}
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	var pageTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(appFS(),
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
		History     []RecordsModerationLogsEntry
		CurrentPage string
		Page        int
		TotalPages  int
		TotalCount  int
	}{
		App:         app,
		User:        admin,
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
	admin, err := requireAdminUser(w, r, h.adminRepo)
	if err != nil {
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
		ModerationStatus string `json:"action"` // "approve", "reject", "flag"
		Reason           string `json:"reason"`
	}

	if err := requireJSONBody(w, r, &req); err != nil {
		return
	}

	// Get the version name before moderation (for logging)
	versionName := ""

	// Try to get pending version name first
	var pendingName string
	err = h.moderationRepo.(*PostgresModerationRepository).db.QueryRowContext(ctx,
		`SELECT name FROM records_revisions
		 WHERE record_id = $1 AND moderation_status = $2 AND change_type = 'PENDING_VERSION'
		 ORDER BY version DESC LIMIT 1`,
		id, StatusPending).Scan(&pendingName)
	if err == nil {
		versionName = pendingName
	} else {
		// No pending version, get main record name
		err = h.moderationRepo.(*PostgresModerationRepository).db.QueryRowContext(ctx,
			`SELECT name FROM records WHERE id = $1`,
			id,
		).Scan(&versionName)
		if err != nil {
			errorLogger.Printf("Error getting record name for logging: %v", err)
			versionName = "" // Continue anyway
		}
	}

	// Validate action
	var newStatus ModerationStatus

	uploaderOrcid, err := h.recordRepo.GetOwnerOrcid(ctx, id)

	switch req.ModerationStatus {
	case "approve":
		newStatus = StatusApproved

		// Check if there's a pending version to approve
		if err := h.moderationRepo.ApprovePendingVersion(ctx, id); err != nil {
			errorLogger.Printf("handler: failed to moderate: %v", err)
			http.Error(w, "Error approving record/version", http.StatusInternalServerError)
			return
		}
		if err := h.notificationService.CreateForRecordModeration(ctx, id, uploaderOrcid, StatusApproved); err != nil {
			errorLogger.Printf("moderation handler: case approved: %v", err)
		}
	case "reject":
		newStatus = StatusRejected
		// Check if there's a pending version to reject
		if err := h.moderationRepo.RejectPendingVersion(ctx, id); err != nil {
			http.Error(w, "Error rejecting record/version", http.StatusInternalServerError)
			return
		}
		if err := h.notificationService.CreateForRecordModeration(ctx, id, uploaderOrcid, StatusRejected); err != nil {
			errorLogger.Printf("moderation handler: case rejected: %v", err)
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
	action := RecordsModerationLogs{
		RecordID:         id,
		AdminOrcid:       admin.Orcid,
		ModerationStatus: newStatus,
		Reason:           req.Reason,
		VersionName:      versionName,
	}
	if err := h.moderationRepo.LogRecordsModerationLogs(ctx, action); err != nil {
		errorLogger.Printf("Error logging moderation action: %v", err)
		// Don't fail the request if logging fails
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  newStatus,
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
		status := http.StatusForbidden
		res := APIResponse[Record]{
			Data: []Record{},
			Meta: ResponseMeta{
				Error: ResponseError{
					Code:        status,
					Message:     http.StatusText(status),
					Description: "you are not allowed to perform this action",
				},
			},
		}
		writeJson(w, res.Meta.Error.Code, res)
	}
}
