package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type CommentHandler struct {
	commentRepo              CommentRepository
	recordRepo               RecordRepository
	adminRepo                AdminRepository
	notificationService      *NotificationService
	commentModerationService *CommentModerationService
}

type createCommentRequest struct {
	Content string `json:"content"`
}

func NewCommentHandler(commentRepo CommentRepository, recordRepo RecordRepository, adminRepo AdminRepository, notificationService *NotificationService, commentModerationService *CommentModerationService) *CommentHandler {
	return &CommentHandler{
		commentRepo:              commentRepo,
		recordRepo:               recordRepo,
		adminRepo:                adminRepo,
		notificationService:      notificationService,
		commentModerationService: commentModerationService,
	}
}

const commentHandlerErr = "Error: comment handler:"
const commentMaxLength = 5000

type deletionMode int

const (
	unknownMode deletionMode = iota
	deleteAsAuthor
	deleteAsAdmin
)

var commentsPath = pathConfig{
	prefix:   "/records/",
	suffix:   "/comments",
	resource: "comment",
}

func (h *CommentHandler) createComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := requireAuthenticatedUser(w, r)
	if err != nil {
		return
	}
	recordId, err := parsePath(w, r, commentsPath)
	if err != nil {
		return
	}
	record, err := h.recordRepo.GetByID(ctx, recordId)
	if err != nil {
		errorLogger.Printf("%s failed to get record %s %v", commentHandlerErr, recordId, err)
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}
	var req createCommentRequest
	if err := requireJSONBody(w, r, &req); err != nil {
		return
	}
	content, err := enforceLength(req.Content, commentMaxLength)
	if err != nil {
		errorLogger.Printf("%s %v", commentHandlerErr, err)
		http.Error(w, fmt.Sprintf("Error: comment must not be empty and must not exceed %d characters", commentMaxLength), http.StatusBadRequest)
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
		errorLogger.Printf("%s failed to create comment for record %q: %v", commentHandlerErr, recordId, err)
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}
	if err := h.notificationService.CreateForComment(ctx, comment, StatusPending); err != nil {
		errorLogger.Printf("%s failed to create comment notification for comment %d: %v", commentHandlerErr, comment.ID, err)
	}
	writeJson(w, http.StatusCreated, comment)
}

func (h *CommentHandler) getComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	recordId, err := parsePath(w, r, commentsPath)
	if err != nil {
		return
	}
	user, isAuthenticated := userFromSession(ctx)
	isAdmin, err := currentUserIsAdmin(w, r, h.adminRepo)
	if err != nil {
		return
	}
	var comments []Comment
	switch {
	case isAdmin:
		comments, err = h.commentRepo.GetByRecordID(ctx, recordId)
	case isAuthenticated:
		comments, err = h.commentRepo.GetVisibleByRecordID(ctx, recordId, user.Orcid)
	default:
		comments, err = h.commentRepo.GetApprovedByRecordID(ctx, recordId)
	}
	if err != nil {
		errorLogger.Printf("%s failed to get comments for record %q: %v", commentHandlerErr, recordId, err)
		http.Error(w, "failed to get comments", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []Comment{}
	}
	writeJson(w, http.StatusOK, comments)
}

func (h *CommentHandler) getPendingComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, err := requireAdminUser(w, r, h.adminRepo)
	if err != nil {
		return
	}
	limit, offset := parsePagination(r)
	total, err := h.commentRepo.CountPending(ctx)
	if err != nil {
		errorLogger.Printf("%s failed to count pending comments: %v", commentHandlerErr, err)
		http.Error(w, "failed to get pending comments count", http.StatusInternalServerError)
		return
	}
	pendingComments, err := h.commentRepo.GetPending(ctx, limit, offset)
	if err != nil {
		errorLogger.Printf("%s failed to get pending comments: %v", commentHandlerErr, err)
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
	writeJson(w, http.StatusOK, comments)
}

func (h *CommentHandler) requireCommentByID(w http.ResponseWriter, r *http.Request, commentID int64) (*Comment, error) {
	comment, err := h.commentRepo.GetByID(r.Context(), commentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "comment not found", http.StatusNotFound)
			return nil, err
		}
		errorLogger.Printf("%s failed to get comment %d: %v", commentHandlerErr, commentID, err)
		http.Error(w, "failed to get comment", http.StatusInternalServerError)
		return nil, err
	}
	return comment, nil
}

func (h *CommentHandler) createApprovedNotifications(ctx context.Context, commentID int64) error {
	comment, err := h.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("%s failed to get comment %d: %w", commentHandlerErr, commentID, err)
	}
	if err := h.notificationService.CreateForCommentModeration(ctx, comment, StatusApproved); err != nil {
		errorLogger.Printf("%s failed to create moderation notification for comment %d: %v", commentHandlerErr, commentID, err)
	}
	recordOwner, err := h.recordRepo.GetOwnerOrcid(ctx, comment.RecordID)
	if err != nil {
		return fmt.Errorf("%s failed to get owner orcid for record %s %w", commentHandlerErr, comment.RecordID, err)
	}
	commentOwner := comment.CommenterOrcid
	if commentOwner != recordOwner {
		if err := h.notificationService.CreateForApprovedComment(ctx, recordOwner, comment, "a new comment has been posted on your record", "posted on your record"); err != nil {
			errorLogger.Printf("%s failed to create record owner notification for cpmment %d: %v", commentHandlerErr, commentID, err)
		}
	}
	commentators, err := h.commentRepo.GetAllOrcids(ctx, comment.RecordID)
	if err != nil {
		errorLogger.Printf("%s failed to get commentators for record: %v", commentHandlerErr, err)
		return nil
	}
	for _, commentator := range commentators {
		if commentator != commentOwner && commentator != recordOwner {
			if err := h.notificationService.CreateForApprovedComment(ctx, commentator, comment, "new activity on a record you follow", "posted on a record you previously commented on"); err != nil {
				errorLogger.Printf("%s failed to create other commentator notification: %v", commentHandlerErr, err)
			}
		}
	}
	return nil
}

func (h *CommentHandler) approveComment(w http.ResponseWriter, r *http.Request) {
	h.moderateComment(w, r, StatusApproved)
}

func (h *CommentHandler) rejectComment(w http.ResponseWriter, r *http.Request) {
	h.moderateComment(w, r, StatusRejected)
}

func (h *CommentHandler) moderateComment(w http.ResponseWriter, r *http.Request, status ModerationStatus) {
	ctx := r.Context()
	admin, err := requireAdminUser(w, r, h.adminRepo)
	if err != nil {
		return
	}
	params, err := requireCommentPathParams(w, r)
	if err != nil {
		return
	}
	commentID := params.commentID
	comment, err := h.requireCommentByID(w, r, commentID)
	if err != nil {
		return
	}
	if err := h.commentModerationService.moderate(ctx, comment, admin.Orcid, status); err != nil {
		errorLogger.Printf("%s failed to moderate comment %d: %v", commentHandlerErr, commentID, err)
		http.Error(w, "failed to approve/reject comment", http.StatusInternalServerError)
		return
	}
	switch status {
	case StatusApproved:
		err = h.createApprovedNotifications(ctx, commentID)
	case StatusRejected:
		err = h.notificationService.CreateForCommentModeration(ctx, comment, status)
	}
	if err != nil {
		errorLogger.Printf("%s failed to create notification for comment %d: %v", commentHandlerErr, commentID, err)
	}
	writeJson(w, http.StatusOK, map[string]ModerationStatus{"status": status})
}

func (h *CommentHandler) flagComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := requireAuthenticatedUser(w, r)
	if err != nil {
		return
	}
	params, err := requireCommentPathParams(w, r)
	if err != nil {
		return
	}
	recordID := params.recordID
	commentID := params.commentID
	comment, err := h.requireCommentByID(w, r, commentID)
	if err != nil {
		return
	}
	if comment.RecordID != recordID {
		errorLogger.Printf("%s comment %d does not belong to record %q", commentHandlerErr, commentID, recordID)
		http.Error(w, "comment does not belong to record", http.StatusNotFound)
		return
	}
	if err := h.commentModerationService.moderate(ctx, comment, user.Orcid, StatusFlagged); err != nil {
		errorLogger.Printf("%s failed to flag comment %d: %v", commentHandlerErr, commentID, err)
		http.Error(w, "failed to flag comment", http.StatusInternalServerError)
		return
	}
	if err := h.notificationService.CreateForComment(ctx, comment, StatusFlagged); err != nil {
		errorLogger.Printf("%s failed to create comment notification for comment %d: %v", commentHandlerErr, comment.ID, err)
	}
	writeJson(w, http.StatusOK, map[string]ModerationStatus{"status": StatusFlagged})
}

func (h *CommentHandler) deleteOwnComment(w http.ResponseWriter, r *http.Request) {
	h.deleteComment(w, r, deleteAsAuthor)
}

func (h *CommentHandler) deleteCommentAsModerator(w http.ResponseWriter, r *http.Request) {
	h.deleteComment(w, r, deleteAsAdmin)
}

// For now, comment deletion is intentionally permanent and is not recorded in the
// moderation log. Revisit this behavior when deletion auditing is introduced.
func (h *CommentHandler) deleteComment(w http.ResponseWriter, r *http.Request, mode deletionMode) {
	ctx := r.Context()
	var user *User
	var err error
	switch mode {
	case deleteAsAuthor:
		user, err = requireAuthenticatedUser(w, r)
	case deleteAsAdmin:
		user, err = requireAdminUser(w, r, h.adminRepo)
	default:
		errorLogger.Printf("%s unsupported comment deletion mode: %d", commentHandlerErr, mode)
		http.Error(w, "unsupported comment deletion mode", http.StatusInternalServerError)
		return
	}
	if err != nil {
		return
	}
	params, err := requireCommentPathParams(w, r)
	if err != nil {
		return
	}
	recordID := params.recordID
	commentID := params.commentID

	comment, err := h.requireCommentByID(w, r, commentID)
	if err != nil {
		return
	}
	if recordID != "" && comment.RecordID != recordID {
		errorLogger.Printf("%s comment %d does not belong to record %q", commentHandlerErr, commentID, recordID)
		http.Error(w, "comment does not belong to record", http.StatusNotFound)
		return
	}

	switch mode {
	case deleteAsAuthor:
		if comment.CommenterOrcid != user.Orcid {
			errorLogger.Printf("%s user %q tried to delete comment %d owned by %q", commentHandlerErr, user.Orcid, commentID, comment.CommenterOrcid)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		err = h.commentRepo.AuthorDeleteComment(ctx, commentID, user.Orcid)
	case deleteAsAdmin:
		err = h.commentRepo.DeleteComment(ctx, commentID)
	}
	if err != nil {
		errorLogger.Printf("%s failed to delete comment %d: %v", commentHandlerErr, commentID, err)
		http.Error(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}
	writeJson(w, http.StatusOK, map[string]ModerationStatus{"status": StatusDeleted})
}
