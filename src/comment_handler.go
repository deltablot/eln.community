package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type CommentHandler struct {
	commentRepo         CommentRepository
	recordRepo          RecordRepository
	adminRepo           AdminRepository
	notificationService *NotificationService
}

type createCommentRequest struct {
	Content string `json:"content"`
}

func NewCommentHandler(commentRepo CommentRepository, recordRepo RecordRepository, adminRepo AdminRepository, notificationService *NotificationService) *CommentHandler {
	return &CommentHandler{
		commentRepo:         commentRepo,
		recordRepo:          recordRepo,
		adminRepo:           adminRepo,
		notificationService: notificationService,
	}
}

const handler = "Error: comment handler"

func requireJSONBody(w http.ResponseWriter, r *http.Request, source string, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
        errorLogger.Printf("%s: invalid request body: %v", source, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
    return true
}

func (h *CommentHandler) createComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

    user, ok := requireAuthenticatedUser(w, r, handler)
    if !ok { return }

    recordId, ok := parsePath(w, r, "/records/", "/comments", "comment", handler)
    if !ok { return }

    record, err := h.recordRepo.GetByID(ctx, recordId)
    if err != nil {
        errorLogger.Printf("%s: failed to get record %s: %v", handler, recordId, err)
	    http.Error(w, "record not found", http.StatusNotFound)
        return
    }

    var req createCommentRequest
    if ok := requireJSONBody(w, r, handler, &req); !ok { return }

    content, ok := requireValidCommentContent(w, r, handler, req.Content)
    if !ok { return }

	comment := &Comment{
		RecordID:         record.Id,
		CommenterName:    user.Name,
		CommenterOrcid:   user.Orcid,
		Content:          content,
		ModerationStatus: StatusPending,
	}

	if err := h.commentRepo.Create(ctx, comment); err != nil {
        errorLogger.Printf("%s: failed to create comment for record %d: %v", handler, recordId, err)
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(comment); err != nil {
    if !ok { return }
        errorLogger.Printf("%s: failed to encode response for comment %d: %v", handler, comment.ID, err)
    }

	if err := h.notificationService.CreateForComment(ctx, comment); err != nil {
		errorLogger.Printf("%s: failed to create comment notification for comment %d: %v", handler, comment.ID, err)
	}
}

func (h *CommentHandler) getComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    recordId, ok := parsePath(w, r, "/records/", "/comments", "comment", handler)
    if !ok { return }

	isAdmin, ok := currentUserIsAdmin(w, r, handler, h.adminRepo)
    if !ok { return }

	var comments []Comment
	var err error
	if isAdmin {
		comments, err = h.commentRepo.GetByRecordID(ctx, recordId)
	} else {
		comments, err = h.commentRepo.GetApprovedByRecordID(ctx, recordId)
	}
	if err != nil {
        errorLogger.Printf("%s: failed to get comments for record %q: %v", handler, recordId, err)
		http.Error(w, "failed to get comments", http.StatusInternalServerError)
		return
	}

	if comments == nil {
		comments = []Comment{}
	}

	w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(comments); err != nil {
        errorLogger.Printf("%s: failed to encode comment response for record %q: %v", handler, recordId, err)
    }
}

func (h *CommentHandler) getPendingComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    _, ok := requireAdminUser(w, r, handler, h.adminRepo)
    if !ok { return }

    limit, offset := parsePagination(r)

	total, err := h.commentRepo.CountPending(ctx)
    if err != nil {
		errorLogger.Printf("Failed to count pending comments: %v", err)
		http.Error(w, "Failed to get pending comments count", http.StatusInternalServerError)
    }
	pendingComments, err := h.commentRepo.GetPending(ctx, limit, offset)
	if err != nil {
		errorLogger.Printf("Failed to get pending comments: %v", err)
		http.Error(w, "Failed to get pending comments", http.StatusInternalServerError)
		return
	}
    if pendingComments == nil {
        pendingComments = []Comment{}
    }

	comments := struct {
		Comments []Comment `json:"comments"`
		Total    int       `json:"total"`
		Limit    int       `json:"limit"`
		Offset   int       `json:"offset"`
	}{
		Comments: pendingComments,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(comments); err != nil {
        errorLogger.Printf("%s: failed to encode pending comment response: %v", handler, err)
    }
}

func (h *CommentHandler) createApprovedNotifications(ctx context.Context, commentID int64) error {
	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("%s: failed to get comment %d: %w", handler, commentID, err)
	}

	if err := h.notificationService.CreateForCommentModeration(ctx, comment, StatusApproved); err != nil {
		errorLogger.Printf("%s: failed to create moderation notification for comment %d: %w", handler, commentID, err)
	}

	recordOwner, err := h.recordRepo.GetOwnerOrcid(ctx, comment.RecordID)
	if err != nil {
		return fmt.Errorf("%s: failed to get owner orcid for record %s: %w", handler, comment.RecordID, err)
	}

    commentOwner := comment.CommenterOrcid
	if commentOwner != recordOwner {
		if err := h.notificationService.CreateForApprovedComment(ctx, recordOwner, comment, "a new comment has been posted on your record", "posted on your record"); err != nil {
			errorLogger.Printf("%s: failed to create record owner notification for cpmment %d: %w", handler, commentID,  err)
		}
	}
	commentators, err := h.commentRepo.GetAllOrcids(ctx, comment.RecordID)
	if err != nil {
		errorLogger.Printf("%s: failed to get commentators for record: %w", handler, err)
        return nil
	}
	for _, commentator := range commentators {
		if commentator != commentOwner && commentator != recordOwner {
			if err := h.notificationService.CreateForApprovedComment(ctx, commentator, comment, "new activity on a record you follow", "posted on a record you previously commented on"); err != nil {
				errorLogger.Printf("%s: failed to create other commentator notification: %w", handler, err)
			}
		}
	}
	return nil
}

// POST /api/v1/moderation/comments/{id}/approve - Approve a comment (admin only)
func (h *CommentHandler) approveComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    admin, ok := requireAdminUser(w, r, handler, h.adminRepo)
    if !ok { return }

    commentPath, ok := parsePath(w, r, "/moderation/comments/", "/approve", "comment moderation", handler)
    if !ok { return }

	commentID, err := strconv.ParseInt(commentPath, 10, 64)
	if err != nil {
        errorLogger.Printf("%s: invalid comment id %q: %v", handler, commentPath, err)
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
    if ok := requireJSONBody(w, r, handler, &req); !ok { return }

    comment, err := h.commentRepo.GetByID(ctx, commentID)
    if err != nil {
        errorLogger.Printf("%s: failed to get comment %d before approval: %v", handler, commentID, err)
		http.Error(w, "failed to approve comment", http.StatusInternalServerError)
		return
    }
    previousStatus := comment.ModerationStatus
	if err := h.commentRepo.MarkAsApproved(ctx, commentID); err != nil {
		http.Error(w, "Failed to approve comment", http.StatusInternalServerError)
		return
	}

	commentModeration := CommentModerationHistory{
		CommentID:  commentID,
		AdminOrcid: admin.Orcid,
        PreviousStatus: previousStatus,
		NewStatus:  StatusApproved,
		Reason: sql.NullString{
			String: req.Reason,
			Valid:  req.Reason != "",
		},
	}
    if err := h.commentRepo.CreateModerationHistory(ctx, commentModeration); err != nil {
        errorLogger.Printf("%s: failed to create moderation history for approved comment %d: %v", handler, commentID, err)
		http.Error(w, "failed to approve comment", http.StatusInternalServerError)
		return
	}

	if err := h.createApprovedNotifications(ctx, commentID); err != nil {
		errorLogger.Printf("%s: failed to create notification for approved comment: %v", handler, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(map[string]string{"status": "approved"}); err != nil {
        errorLogger.Printf("%s: failed to encode moderation status approved comment response: %v", handler, err)
    }
}

// POST /api/v1/moderation/comments/{id}/reject - Reject a comment (admin only)
func (h *CommentHandler) rejectComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    admin, ok := requireAdminUser(w, r, handler, h.adminRepo)
    if !ok {
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
		AdminOrcid: admin.Orcid,
		NewStatus:  StatusRejected,
		Reason: sql.NullString{
			String: req.Reason,
			Valid:  req.Reason != "",
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
		NewStatus:  StatusDeleted,
		Reason: sql.NullString{
			String: req.Reason,
			Valid:  req.Reason != "",
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
