package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

const commentHandlerErr = "comment handler:"

func (h *CommentHandler) createComment(w http.ResponseWriter, r *http.Request) {
	source := errorSource("CreateComment", commentHandlerErr)
	ctx := r.Context()
	user, ok := requireAuthenticatedUser(w, r, source)
	if !ok {
		return
	}
	recordId, ok := parsePath(w, r, "/records/", "/comments", "comment", source)
	if !ok {
		return
	}
	record, err := h.recordRepo.GetByID(ctx, recordId)
	if err != nil {
		errorLogger.Printf("%s failed to get record %s %v", source, recordId, err)
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}
	var req createCommentRequest
	if ok := requireJSONBody(w, r, source, &req); !ok {
		return
	}
	content, ok := requireValidCommentContent(w, r, source, req.Content)
	if !ok {
		return
	}
	comment := &Comment{
		RecordID:         record.Id,
		CommenterName:    user.Name,
		CommenterOrcid:   user.Orcid,
		Content:          content,
		ModerationStatus: StatusPending,
	}
	if err := h.commentRepo.Create(ctx, comment); err != nil {
		errorLogger.Printf("%s failed to create comment for record %q: %v", source, recordId, err)
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}
	writeJson(w, source, http.StatusCreated, comment)
	if err := h.notificationService.CreateForComment(ctx, comment, StatusPending); err != nil {
		errorLogger.Printf("%s failed to create comment notification for comment %d: %v", source, comment.ID, err)
	}
}

func (h *CommentHandler) getComments(w http.ResponseWriter, r *http.Request) {
	source := errorSource("getComments", commentHandlerErr)
	ctx := r.Context()
	recordId, ok := parsePath(w, r, "/records/", "/comments", "comment", source)
	if !ok {
		return
	}
	user, isAuthenticated := userFromSession(ctx)

	isAdmin, ok := currentUserIsAdmin(w, r, source, h.adminRepo)
	if !ok {
		return
	}
	var comments []Comment
	var err error
	switch {
	case isAdmin:
		comments, err = h.commentRepo.GetByRecordID(ctx, recordId)
	case isAuthenticated:
		comments, err = h.commentRepo.GetVisibleByRecordID(ctx, recordId, user.Orcid)
	default:
		comments, err = h.commentRepo.GetApprovedByRecordID(ctx, recordId)
	}
	if err != nil {
		errorLogger.Printf("%s failed to get comments for record %q: %v", source, recordId, err)
		http.Error(w, "failed to get comments", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []Comment{}
	}
	writeJson(w, source, http.StatusOK, comments)
}

func (h *CommentHandler) getPendingComments(w http.ResponseWriter, r *http.Request) {
	source := errorSource("getPendingComments", commentHandlerErr)
	ctx := r.Context()
	_, ok := requireAdminUser(w, r, source, h.adminRepo)
	if !ok {
		return
	}
	limit, offset := parsePagination(r)
	total, err := h.commentRepo.CountPending(ctx)
	if err != nil {
		errorLogger.Printf("%s failed to count pending comments: %v", source, err)
		http.Error(w, "failed to get pending comments count", http.StatusInternalServerError)
		return
	}
	pendingComments, err := h.commentRepo.GetPending(ctx, limit, offset)
	if err != nil {
		errorLogger.Printf("%s failed to get pending comments: %v", source, err)
		http.Error(w, "failed to get pending comments", http.StatusInternalServerError)
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
	writeJson(w, source, http.StatusOK, comments)
}

func (h *CommentHandler) createApprovedNotifications(ctx context.Context, commentID int64) error {
	source := errorSource("createApprovedNotifications", commentHandlerErr)
	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("%s failed to get comment %d: %w", source, commentID, err)
	}
	if err := h.notificationService.CreateForCommentModeration(ctx, comment, StatusApproved); err != nil {
		errorLogger.Printf("%s failed to create moderation notification for comment %d: %v", source, commentID, err)
	}
	recordOwner, err := h.recordRepo.GetOwnerOrcid(ctx, comment.RecordID)
	if err != nil {
		return fmt.Errorf("%s failed to get owner orcid for record %s %w", source, comment.RecordID, err)
	}
	commentOwner := comment.CommenterOrcid
	if commentOwner != recordOwner {
		if err := h.notificationService.CreateForApprovedComment(ctx, recordOwner, comment, "a new comment has been posted on your record", "posted on your record"); err != nil {
			errorLogger.Printf("%s failed to create record owner notification for cpmment %d: %v", source, commentID, err)
		}
	}
	commentators, err := h.commentRepo.GetAllOrcids(ctx, comment.RecordID)
	if err != nil {
		errorLogger.Printf("%s failed to get commentators for record: %v", source, err)
		return nil
	}
	for _, commentator := range commentators {
		if commentator != commentOwner && commentator != recordOwner {
			if err := h.notificationService.CreateForApprovedComment(ctx, commentator, comment, "new activity on a record you follow", "posted on a record you previously commented on"); err != nil {
				errorLogger.Printf("%s failed to create other commentator notification: %v", source, err)
			}
		}
	}
	return nil
}

func (h *CommentHandler) moderateComment(w http.ResponseWriter, r *http.Request, suffix string, status ModerationStatus) {
	source := errorSource("moderateComment", commentHandlerErr)
	ctx := r.Context()
	admin, ok := requireAdminUser(w, r, source, h.adminRepo)
	if !ok {
		return
	}
	commentPath, ok := parsePath(w, r, "/moderation/comments/", "/"+suffix, "comment moderation", source)
	if !ok {
		return
	}
	commentID, err := strconv.ParseInt(commentPath, 10, 64)
	if err != nil {
		errorLogger.Printf("%s invalid comment id %q: %v", source, commentPath, err)
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}

	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		errorLogger.Printf("%s failed to get comment %d before approval/rejection: %v", source, commentID, err)
		http.Error(w, "failed to moderate comment", http.StatusInternalServerError)
		return
	}
	if status == StatusApproved {
		if err := h.commentRepo.MarkAsApproved(ctx, commentID); err != nil {
			http.Error(w, "failed to approve comment", http.StatusInternalServerError)
			return
		}
	}
	if status == StatusRejected {
		if err := h.commentRepo.MarkAsRejected(ctx, commentID); err != nil {
			http.Error(w, "failed to reject comment", http.StatusInternalServerError)
			return
		}
	}
	commentModeration := CommentModerationHistory{
		CommentID:      commentID,
		ReporterOrcid:  admin.Orcid,
		PreviousStatus: comment.ModerationStatus,
		NewStatus:      status,
	}
	if err := h.commentRepo.CreateModerationHistory(ctx, commentModeration); err != nil {
		errorLogger.Printf("%s failed to create moderation history for %d comment %d: %v", source, status, commentID, err)
		http.Error(w, "failed to approve/reject comment", http.StatusInternalServerError)
		return
	}
	if status == StatusApproved {
		err = h.createApprovedNotifications(ctx, commentID)
	}
	if status == StatusRejected {
		err = h.notificationService.CreateForCommentModeration(ctx, comment, StatusRejected)
	}
	if err != nil {
		errorLogger.Printf("%s failed to create notification for %s comment: %v", source, status, err)
	}
	writeJson(w, source, http.StatusOK, map[string]string{"status": suffix})
}

func (h *CommentHandler) flagComment(w http.ResponseWriter, r *http.Request) {
	source := errorSource("flagComment", commentHandlerErr)
	ctx := r.Context()
	user, ok := requireAuthenticatedUser(w, r, source)
	if !ok {
		return
	}
	recordID := r.PathValue("recordID")
	commentIDStr := r.PathValue("commentID")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		errorLogger.Printf("%s invalid comment id %q: %v", source, commentIDStr, err)
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}

	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		errorLogger.Printf("%s failed to get comment %d before flagging: %v", source, commentID, err)
		http.Error(w, "failed to flag comment", http.StatusInternalServerError)
		return
	}
	if comment.RecordID != recordID {
		errorLogger.Printf("%s comment %s does not belong to record %q: %v", source, commentID, recordID, err)
		http.Error(w, "comment does not belong to record", http.StatusInternalServerError)
		return
	}
	if err := h.commentRepo.MarkAsFlagged(ctx, commentID); err != nil {
		errorLogger.Printf("%s failed to flag comment %d: %v", source, commentID, err)
		http.Error(w, "failed to flag comments", http.StatusInternalServerError)
		return
	}
	commentModeration := CommentModerationHistory{
		CommentID:      commentID,
		ReporterOrcid:  user.Orcid,
		PreviousStatus: comment.ModerationStatus,
		NewStatus:      StatusFlagged,
	}
	if err := h.commentRepo.CreateModerationHistory(ctx, commentModeration); err != nil {
		errorLogger.Printf("%s failed to create moderation history for flagged comment %d: %v", source, commentID, err)
		http.Error(w, "failed to flag comment", http.StatusInternalServerError)
		return
	}

	writeJson(w, source, http.StatusOK, map[string]any{"status": StatusFlagged})
	if err := h.notificationService.CreateForComment(ctx, comment, StatusFlagged); err != nil {
		errorLogger.Printf("%s failed to create comment notification for comment %d: %v", source, comment.ID, err)
	}
}

func (h *CommentHandler) deleteComment(w http.ResponseWriter, r *http.Request) {
	source := errorSource("deleteComment", commentHandlerErr)
	ctx := r.Context()
	user, ok := requireAuthenticatedUser(w, r, source)
	if !ok {
		return
	}
	recordID := r.PathValue("recordID")
	commentIDStr := r.PathValue("commentID")
	isModerationRoute := false
	if commentIDStr == "" {
		commentIDStr = r.PathValue("id")
		isModerationRoute = true
	}
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		errorLogger.Printf("%s invalid comment id %q: %v", source, commentIDStr, err)
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}

	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		errorLogger.Printf("%s failed to get comment %d before deletion: %v", source, commentID, err)
		http.Error(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}
	if recordID != "" && comment.RecordID != recordID {
		errorLogger.Printf("%s comment %s does not belong to record %q: %v", source, commentID, recordID, err)
		http.Error(w, "comment does not belong to record", http.StatusInternalServerError)
		return
	}
	isAdmin, ok := currentUserIsAdmin(w, r, source, h.adminRepo)
	if !ok {
		return
	}
	if isModerationRoute && !isAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !isAdmin && comment.CommenterOrcid != user.Orcid {
		errorLogger.Printf("%s user %q tried to delete comment %d owned by %q", source, user.Orcid, commentID, comment.CommenterOrcid)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	commentModeration := CommentModerationHistory{
		CommentID:      commentID,
		ReporterOrcid:  user.Orcid,
		PreviousStatus: comment.ModerationStatus,
		NewStatus:      StatusDeleted,
	}
	if err := h.commentRepo.CreateModerationHistory(ctx, commentModeration); err != nil {
		errorLogger.Printf("%s failed to create moderation history for deleted comment %d: %v", source, commentID, err)
		http.Error(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}
	if isAdmin {
		err = h.commentRepo.DeleteComment(ctx, commentID)
	} else {
		err = h.commentRepo.AuthorDeleteComment(ctx, commentID, user.Orcid)
	}
	if err != nil {
		errorLogger.Printf("%s failed to delete comment %d: %v", source, commentID, err)
		http.Error(w, "failed to delete comments", http.StatusInternalServerError)
		return
	}
	writeJson(w, source, http.StatusOK, map[string]any{"status": StatusDeleted})
}
