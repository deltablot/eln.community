package app

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type CommentHandler struct {
	commentRepo CommentRepository
	recordRepo  RecordRepository
	adminRepo   AdminRepository
}

func NewCommentHandler(commentRepo CommentRepository, recordRepo RecordRepository, adminRepo AdminRepository) *CommentHandler {
	return &CommentHandler{
		commentRepo: commentRepo,
		recordRepo:  recordRepo,
		adminRepo:   adminRepo,
	}
}

// POST /api/v1/records/{id}/comments - Create a new comment
func (h *CommentHandler) createComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Authentication required
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok || orcid == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	userName, _ := sessionManager.Get(ctx, "name").(string)
	user := &User{
		Name:  userName,
		Orcid: orcid,
	}

	// Get record ID from URL
	recordID := strings.TrimPrefix(r.URL.Path, "/api/v1/records/")
	recordID = strings.TrimSuffix(recordID, "/comments")

	// Verify record exists
	record, err := h.recordRepo.GetByID(ctx, recordID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate content
	if len(strings.TrimSpace(req.Content)) == 0 {
		http.Error(w, "Comment content cannot be empty", http.StatusBadRequest)
		return
	}

	if len(req.Content) > 5000 {
		http.Error(w, "Comment content too long (max 5000 characters)", http.StatusBadRequest)
		return
	}

	// Check if moderation is enabled
	moderationEnabled := os.Getenv("MODERATION_ENABLED") != "false"
	initialStatus := StatusApproved
	if moderationEnabled {
		initialStatus = StatusPendingReview
	}

	// Create comment
	comment := &Comment{
		RecordID:         record.Id,
		CommenterName:    user.Name,
		CommenterOrcid:   user.Orcid,
		Content:          req.Content,
		ModerationStatus: initialStatus,
	}

	if err := h.commentRepo.Create(ctx, comment); err != nil {
		errorLogger.Printf("Failed to create comment: %v", err)
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

// GET /api/v1/records/{id}/comments - Get comments for a record
func (h *CommentHandler) getComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get record ID from URL
	recordID := strings.TrimPrefix(r.URL.Path, "/api/v1/records/")
	recordID = strings.TrimSuffix(recordID, "/comments")

	// Check if user is admin (can see all comments including pending)
	isAdmin := false
	if orcid, ok := sessionManager.Get(ctx, "orcid").(string); ok && orcid != "" {
		isAdmin, _ = h.adminRepo.IsAdmin(ctx, orcid)
	}

	// Get comments
	comments, err := h.commentRepo.GetByRecordID(ctx, recordID, isAdmin)
	if err != nil {
		errorLogger.Printf("Failed to get comments: %v", err)
		http.Error(w, "Failed to get comments", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array instead of null if no comments
	if comments == nil {
		comments = []Comment{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// GET /api/v1/moderation/comments - Get pending comments (admin only)
func (h *CommentHandler) getPendingComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Admin check
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok || orcid == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	comments, total, err := h.commentRepo.GetPendingComments(ctx, limit, offset)
	if err != nil {
		errorLogger.Printf("Failed to get pending comments: %v", err)
		http.Error(w, "Failed to get pending comments", http.StatusInternalServerError)
		return
	}

	response := struct {
		Comments []Comment `json:"comments"`
		Total    int       `json:"total"`
		Limit    int       `json:"limit"`
		Offset   int       `json:"offset"`
	}{
		Comments: comments,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// POST /api/v1/moderation/comments/{id}/approve - Approve a comment (admin only)
func (h *CommentHandler) approveComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Admin check
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok || orcid == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Get comment ID
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/moderation/comments/")
	commentIDStr = strings.TrimSuffix(commentIDStr, "/approve")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	// Parse optional reason
	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Approve comment
	if err := h.commentRepo.ApproveComment(ctx, commentID); err != nil {
		errorLogger.Printf("Failed to approve comment: %v", err)
		http.Error(w, "Failed to approve comment", http.StatusInternalServerError)
		return
	}

	// Log moderation action
	action := CommentModerationAction{
		CommentID:  commentID,
		AdminOrcid: orcid,
		Action:     "approve",
		Reason:     req.Reason,
	}
	h.commentRepo.LogModerationAction(ctx, action)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

// POST /api/v1/moderation/comments/{id}/reject - Reject a comment (admin only)
func (h *CommentHandler) rejectComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Admin check
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok || orcid == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Get comment ID
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/moderation/comments/")
	commentIDStr = strings.TrimSuffix(commentIDStr, "/reject")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	// Parse optional reason
	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Reject comment
	if err := h.commentRepo.RejectComment(ctx, commentID); err != nil {
		errorLogger.Printf("Failed to reject comment: %v", err)
		http.Error(w, "Failed to reject comment", http.StatusInternalServerError)
		return
	}

	// Log moderation action
	action := CommentModerationAction{
		CommentID:  commentID,
		AdminOrcid: orcid,
		Action:     "reject",
		Reason:     req.Reason,
	}
	h.commentRepo.LogModerationAction(ctx, action)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
}

// DELETE /api/v1/moderation/comments/{id} - Delete a comment (admin only)
func (h *CommentHandler) deleteComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Admin check
	orcid, ok := sessionManager.Get(ctx, "orcid").(string)
	if !ok || orcid == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil || !isAdmin {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Get comment ID
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/moderation/comments/")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	// Parse optional reason
	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Log moderation action before deletion
	action := CommentModerationAction{
		CommentID:  commentID,
		AdminOrcid: orcid,
		Action:     "delete",
		Reason:     req.Reason,
	}
	h.commentRepo.LogModerationAction(ctx, action)

	// Delete comment
	if err := h.commentRepo.DeleteComment(ctx, commentID); err != nil {
		errorLogger.Printf("Failed to delete comment: %v", err)
		http.Error(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
