package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"database/sql"
)

type CommentHandler struct {
	commentRepo         CommentRepository
	recordRepo          RecordRepository
	adminRepo           AdminRepository
	notificationService *NotificationService
}

func NewCommentHandler(commentRepo CommentRepository, recordRepo RecordRepository, adminRepo AdminRepository, notificationService *NotificationService) *CommentHandler {
	return &CommentHandler{
		commentRepo:         commentRepo,
		recordRepo:          recordRepo,
		adminRepo:           adminRepo,
		notificationService: notificationService,
	}
}

const handler = "comment handler"

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

	// Create comment
	comment := &Comment{
		RecordID:         record.Id,
		CommenterName:    user.Name,
		CommenterOrcid:   user.Orcid,
		Content:          req.Content,
		ModerationStatus: StatusPending,
	}

	if err := h.commentRepo.Create(ctx, comment); err != nil {
		errorLogger.Printf("Failed to create comment: %v", err)
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)

	if err := h.notificationService.CreateForComment(ctx, comment); err != nil {
		errorLogger.Printf("%s: failed to create comment notification: %v", handler, err)
	}
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
	var comments []Comment
	var err error
	if isAdmin {
		comments, err = h.commentRepo.GetByRecordID(ctx, recordID)
	} else {
		comments, err = h.commentRepo.GetApprovedByRecordID(ctx, recordID)
	}
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

    total, err := h.commentRepo.CountPending(ctx)
	comments, err := h.commentRepo.GetPending(ctx, limit, offset)
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

func (h *CommentHandler) createApprovedNotification(ctx context.Context, commentID int64) error {
	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("%s: failed to get comment id: %v", handler, err)
	}

	if err := h.notificationService.CreateForCommentModeration(ctx, comment, StatusApproved); err != nil {
		errorLogger.Printf("%s: failed to create moderation notification: %v", handler, err)
	}

	recordOwner, err := h.recordRepo.GetOwnerOrcid(ctx, comment.RecordID)
	if err != nil {
		return fmt.Errorf("%s: failed to get owner orcid: %v", handler, err)
	}

	commentOwner, err := h.commentRepo.GetCommentatorOrcid(ctx, commentID)
	if err != nil {
		return fmt.Errorf("%s: failed to get commentator orcid: %v", handler, err)
	}

	if commentOwner != recordOwner {
		if err := h.notificationService.CreateForApprovedComment(ctx, recordOwner, comment, "a new comment has been posted on your record", "posted on your record"); err != nil {
			errorLogger.Printf("%s: failed to create record owner notification: %v", handler, err)
		}
	}
	commentators, err := h.commentRepo.GetAllOrcids(ctx, comment.RecordID)
	if err != nil {
		errorLogger.Printf("%s: failed to get commentators for record: %v", handler, err)
	}
	for _, commentator := range commentators {
		if commentator != commentOwner && commentator != recordOwner {
			if err := h.notificationService.CreateForApprovedComment(ctx, commentator, comment, "new activity on a record you follow", "posted on a record you previously commented on"); err != nil {
				errorLogger.Printf("%s: failed to create other commentator notification: %v", handler, err)
			}
		}
	}
	return nil
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
	err1 := h.commentRepo.MarkAsApproved(ctx, commentID)
    if err1 == nil {
		errorLogger.Printf("ICI Failed to approve comment: %v", err1)
    }
    if err1 != nil {
		http.Error(w, "Failed to approve comment", http.StatusInternalServerError)
		return
	}

	// Log moderation action
	action := CommentModerationHistory{
		CommentID:  commentID,
		AdminOrcid: orcid,
		NewStatus:     StatusApproved,
		Reason:     sql.NullString{
            String: req.Reason,
            Valid: req.Reason != "",
        },
	}
	h.commentRepo.CreateModerationHistory(ctx, action)

	if err := h.createApprovedNotification(ctx, commentID); err != nil {
		errorLogger.Printf("%s: failed to create notification for approved comment: %v", handler, err)
	}

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
	if err := h.commentRepo.MarkAsRejected(ctx, commentID); err != nil {
		errorLogger.Printf("Failed to reject comment: %v", err)
		http.Error(w, "Failed to reject comment", http.StatusInternalServerError)
		return
	}

	// Log moderation action
	action := CommentModerationHistory{
		CommentID:  commentID,
		AdminOrcid: orcid,
		NewStatus:     StatusRejected,
		Reason:     sql.NullString{
            String: req.Reason,
            Valid: req.Reason != "",
        },
	}
	h.commentRepo.CreateModerationHistory(ctx, action)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})

	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		errorLogger.Printf("%s: failed to get comment id: %v", handler, err)
		return
	}
	if err := h.notificationService.CreateForCommentModeration(ctx, comment, StatusRejected); err != nil {
		errorLogger.Printf("%s: case rejected: %v", handler, err)
	}

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
	action := CommentModerationHistory{
		CommentID:  commentID,
		AdminOrcid: orcid,
		NewStatus:     StatusDeleted,
		Reason:     sql.NullString{
            String: req.Reason,
            Valid: req.Reason != "",
        },
	}
	h.commentRepo.CreateModerationHistory(ctx, action)

	// Delete comment
	if err := h.commentRepo.DeleteComment(ctx, commentID); err != nil {
		errorLogger.Printf("Failed to delete comment: %v", err)
		http.Error(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
