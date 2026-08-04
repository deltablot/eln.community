package main

import (
	"context"
	"fmt"
)

type CommentModerationService struct {
	commentRepo CommentRepository
}

func NewCommentModerationService(commentRepo CommentRepository) *CommentModerationService {
	return &CommentModerationService{
		commentRepo: commentRepo,
	}
}

func (s *CommentModerationService) setCommentModerationStatus(ctx context.Context, commentID int64, status ModerationStatus) error {
	switch status {
	case StatusApproved:
		if err := s.commentRepo.MarkAsApproved(ctx, commentID); err != nil {
			return fmt.Errorf("approve comment: %w", err)
		}
	case StatusRejected:
		if err := s.commentRepo.MarkAsRejected(ctx, commentID); err != nil {
			return fmt.Errorf("reject comment: %w", err)
		}
	case StatusFlagged:
		if err := s.commentRepo.MarkAsFlagged(ctx, commentID); err != nil {
			return fmt.Errorf("flag comment: %w", err)
		}
	default:
		return fmt.Errorf("unsupported moderation status: %d", status)
	}
	return nil
}

func (s *CommentModerationService) createLog(ctx context.Context, comment *Comment, orcid string, status ModerationStatus) error {
	commentModeration := CommentsModerationLogs{
		CommentID:      comment.ID,
		ReporterOrcid:  orcid,
		PreviousStatus: comment.ModerationStatus,
		NewStatus:      status,
	}
	if err := s.commentRepo.CreateRecordsModerationLogs(ctx, commentModeration); err != nil {
		return fmt.Errorf("create moderation log for comment %d: %w", comment.ID, err)
	}
	return nil
}
