// Package service provides business logic for the reviews module.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/reviews/api/dto"
	"erg.ninja/internal/modules/reviews/domain/entity"
	"erg.ninja/internal/modules/reviews/infrastructure/repository"
	"erg.ninja/pkg/logger"
)

// Service provides reviews business logic.
type Service struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewService creates a new reviews service.
func NewService(repo *repository.Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ─── Public ────────────────────────────────────────────────────────────────────

// CreateReview creates a new review (starts as pending).
func (s *Service) CreateReview(ctx context.Context, req *dto.CreateReviewRequest) (*dto.ReviewResponse, error) {
	review := &entities.Review{
		TargetID:   req.TargetID,
		TargetType: entities.ReviewTargetType(req.TargetType),
		UserName:   req.UserName,
		UserEmail:  req.UserEmail,
		Rating:     req.Rating,
		Comment:    req.Comment,
		Status:     entities.ReviewStatusPending,
	}

	if err := s.repo.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("reviews.CreateReview: %w", err)
	}

	s.log.InfoContext(ctx).
		Str("review_id", review.ID.Hex()).
		Str("target_id", review.TargetID).
		Int("rating", review.Rating).
		Msg("reviews: review created (pending)")

	return s.toReviewResponse(review), nil
}

// ListReviews returns paginated approved reviews with stats.
func (s *Service) ListReviews(ctx context.Context, params dto.ReviewQueryParams) (*dto.ReviewListResponse, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Page < 1 {
		params.Page = 1
	}

	reviews, total, err := s.repo.List(ctx, repository.ListParams{
		TargetID:   params.TargetID,
		TargetType: params.TargetType,
		Page:       params.Page,
		Limit:      params.Limit,
		Sort:       params.Sort,
	})
	if err != nil {
		return nil, fmt.Errorf("reviews.ListReviews: %w", err)
	}

	// Build stats (optional — only when filtering by targetId).
	var stats *dto.ReviewStatsResponse
	if params.TargetID != "" {
		avg, cnt, dist, err := s.repo.AggregateStats(ctx, params.TargetID, params.TargetType)
		if err != nil {
			s.log.WarnContext(ctx).Err(err).Str("target_id", params.TargetID).Msg("reviews: AggregateStats failed, skipping stats")
		} else {
			stats = &dto.ReviewStatsResponse{
				Average:      avg,
				Count:        cnt,
				Distribution: dist,
			}
		}
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit != 0 {
		totalPages++
	}

	return &dto.ReviewListResponse{
		Data: dto.ToReviewResponses(reviews),
		Meta: &dto.ListMeta{
			Total:      total,
			Page:       params.Page,
			Limit:      params.Limit,
			TotalPages: totalPages,
		},
		Stats: stats,
	}, nil
}

// MarkHelpful increments the helpful count for a review.
func (s *Service) MarkHelpful(ctx context.Context, reviewID string) error {
	if err := s.repo.IncrementHelpful(ctx, reviewID); err != nil {
		return fmt.Errorf("reviews.MarkHelpful: %w", err)
	}
	s.log.InfoContext(ctx).Str("review_id", reviewID).Msg("reviews: helpful marked")
	return nil
}

// GetStats returns aggregate rating statistics for a target.
func (s *Service) GetStats(ctx context.Context, targetID, targetType string) (*dto.ReviewStatsResponse, error) {
	avg, cnt, dist, err := s.repo.AggregateStats(ctx, targetID, targetType)
	if err != nil {
		return nil, fmt.Errorf("reviews.GetStats: %w", err)
	}
	return &dto.ReviewStatsResponse{
		Average:      avg,
		Count:        cnt,
		Distribution: dist,
	}, nil
}

// ─── Admin ─────────────────────────────────────────────────────────────────────

// ListAdminReviews returns paginated reviews for admin with optional filters.
func (s *Service) ListAdminReviews(ctx context.Context, params dto.AdminReviewQueryParams) (*dto.AdminReviewListResponse, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Page < 1 {
		params.Page = 1
	}

	reviews, total, err := s.repo.AdminList(ctx, repository.AdminListParams{
		Status:     params.Status,
		TargetType: params.TargetType,
		TargetID:   params.TargetID,
		Page:       params.Page,
		Limit:      params.Limit,
		Sort:       params.Sort,
	})
	if err != nil {
		return nil, fmt.Errorf("reviews.ListAdminReviews: %w", err)
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit != 0 {
		totalPages++
	}

	return &dto.AdminReviewListResponse{
		Data: dto.ToReviewResponses(reviews),
		Meta: &dto.ListMeta{
			Total:      total,
			Page:       params.Page,
			Limit:      params.Limit,
			TotalPages: totalPages,
		},
	}, nil
}

// ApproveReview approves a pending review.
func (s *Service) ApproveReview(ctx context.Context, reviewID, adminUserID string) (*dto.ReviewResponse, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      entities.ReviewStatusApproved,
		"reviewed_by": adminUserID,
		"reviewed_at": now,
	}
	if err := s.repo.UpdateFields(ctx, reviewID, updates); err != nil {
		return nil, fmt.Errorf("reviews.ApproveReview: %w", err)
	}

	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("reviews.ApproveReview fetch: %w", err)
	}

	s.log.InfoContext(ctx).Str("review_id", reviewID).Str("admin", adminUserID).Msg("reviews: review approved")
	return s.toReviewResponse(review), nil
}

// RejectReview rejects a review with a reason.
func (s *Service) RejectReview(ctx context.Context, reviewID, adminUserID string, req *dto.RejectReviewRequest) (*dto.ReviewResponse, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      entities.ReviewStatusRejected,
		"admin_note":  req.Note,
		"reason":      req.Reason,
		"reviewed_by": adminUserID,
		"reviewed_at": now,
	}
	if err := s.repo.UpdateFields(ctx, reviewID, updates); err != nil {
		return nil, fmt.Errorf("reviews.RejectReview: %w", err)
	}

	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("reviews.RejectReview fetch: %w", err)
	}

	s.log.InfoContext(ctx).Str("review_id", reviewID).Str("admin", adminUserID).Str("reason", req.Reason).Msg("reviews: review rejected")
	return s.toReviewResponse(review), nil
}

// ReplyReview adds or updates an admin reply to a review.
func (s *Service) ReplyReview(ctx context.Context, reviewID, adminUserID string, req *dto.ReplyReviewRequest) (*dto.ReviewResponse, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"admin_reply": req.Reply,
		"reply_by":    adminUserID,
		"reply_at":    now,
	}
	if err := s.repo.UpdateFields(ctx, reviewID, updates); err != nil {
		return nil, fmt.Errorf("reviews.ReplyReview: %w", err)
	}

	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("reviews.ReplyReview fetch: %w", err)
	}

	s.log.InfoContext(ctx).Str("review_id", reviewID).Str("admin", adminUserID).Msg("reviews: admin reply added")
	return s.toReviewResponse(review), nil
}

// ToggleFeatured toggles the featured flag on a review.
func (s *Service) ToggleFeatured(ctx context.Context, reviewID string, isFeatured bool) (*dto.ReviewResponse, error) {
	updates := map[string]any{"is_featured": isFeatured}
	if err := s.repo.UpdateFields(ctx, reviewID, updates); err != nil {
		return nil, fmt.Errorf("reviews.ToggleFeatured: %w", err)
	}

	review, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("reviews.ToggleFeatured fetch: %w", err)
	}

	s.log.InfoContext(ctx).Str("review_id", reviewID).Bool("is_featured", isFeatured).Msg("reviews: featured toggled")
	return s.toReviewResponse(review), nil
}

// UpdateStatus maps the legacy status API to approve/reject flows.
func (s *Service) UpdateStatus(ctx context.Context, reviewID, adminUserID, status, adminNote string) (*dto.ReviewResponse, error) {
	switch strings.ToLower(status) {
	case string(entities.ReviewStatusApproved):
		return s.ApproveReview(ctx, reviewID, adminUserID)
	case string(entities.ReviewStatusRejected):
		return s.RejectReview(ctx, reviewID, adminUserID, &dto.RejectReviewRequest{
			Reason: "rejected_by_admin",
			Note:   adminNote,
		})
	default:
		return nil, fmt.Errorf("reviews.UpdateStatus: unsupported status %q", status)
	}
}

// BulkUpdateStatus applies the same moderation status to multiple reviews.
func (s *Service) BulkUpdateStatus(ctx context.Context, reviewIDs []string, adminUserID, status string) (map[string]any, error) {
	updated := make([]*dto.ReviewResponse, 0, len(reviewIDs))
	for _, reviewID := range reviewIDs {
		result, err := s.UpdateStatus(ctx, reviewID, adminUserID, status, "")
		if err != nil {
			return nil, fmt.Errorf("reviews.BulkUpdateStatus[%s]: %w", reviewID, err)
		}
		updated = append(updated, result)
	}
	return map[string]any{
		"updated": len(updated),
		"items":   updated,
	}, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func (s *Service) toReviewResponse(r *entities.Review) *dto.ReviewResponse {
	resp := dto.ToReviewResponse(r)
	return &resp
}
