package main

import (
	"context"
	"database/sql"
	"fmt"
)

type CommentModerationService struct {
	db          *sql.DB
	commentRepo CommentRepository
}

func NewCommentModerationService(db *sql.DB, commentRepo CommentRepository) *CommentModerationService {
	return &CommentModerationService{
		db:          db,
		commentRepo: commentRepo,
	}
}

func (s *CommentModerationService) setCommentModerationStatus(ctx context.Context, tx *sql.Tx, commentID int64, status ModerationStatus) error {
	switch status {
	case StatusApproved:
		if err := s.commentRepo.MarkAsApproved(ctx, tx, commentID); err != nil {
			return fmt.Errorf("approve comment: %w", err)
		}
	case StatusRejected:
		if err := s.commentRepo.MarkAsRejected(ctx, tx, commentID); err != nil {
			return fmt.Errorf("reject comment: %w", err)
		}
	case StatusFlagged:
		if err := s.commentRepo.MarkAsFlagged(ctx, tx, commentID); err != nil {
			return fmt.Errorf("flag comment: %w", err)
		}
	default:
		return fmt.Errorf("unsupported moderation status: %d", status)
	}
	return nil
}

func (s *CommentModerationService) createLog(ctx context.Context, tx *sql.Tx, comment *Comment, orcid string, status ModerationStatus) error {
	commentModeration := CommentsModerationLogs{
		CommentID:      comment.ID,
		ReporterOrcid:  orcid,
		PreviousStatus: comment.ModerationStatus,
		NewStatus:      status,
	}
	if err := s.commentRepo.CreateModerationLogs(ctx, tx, commentModeration); err != nil {
		return fmt.Errorf("create moderation log for comment %d: %w", comment.ID, err)
	}
	return nil
}

func (s *CommentModerationService) moderate(ctx context.Context, comment *Comment, orcid string, status ModerationStatus) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.setCommentModerationStatus(ctx, tx, comment.ID, status); err != nil {
		return err
	}
	if err := s.createLog(ctx, tx, comment, orcid, status); err != nil {
		return err
	}
	return tx.Commit()
}
