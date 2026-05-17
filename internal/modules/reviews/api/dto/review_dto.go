// Package dto defines request/response types for the reviews module.
package dto

import (
	"github.com/go-playground/validator/v10"

	"erg.ninja/internal/modules/reviews/domain/entity"
)

// ─── Request DTOs ─────────────────────────────────────────────────────────────

// CreateReviewRequest mirrors CreateReviewDto from erg-backend.
type CreateReviewRequest struct {
	TargetID   string `json:"target_id" validate:"required"`
	TargetType string `json:"target_type" validate:"required"`
	UserName   string `json:"user_name" validate:"required"`
	UserEmail  string `json:"user_email,omitempty"`
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
	Comment    string `json:"comment" validate:"required"`
}

// RejectReviewRequest mirrors RejectReviewDto from erg-backend.
type RejectReviewRequest struct {
	Reason string `json:"reason" validate:"required"`
	Note   string `json:"note,omitempty"`
}

// ReplyReviewRequest mirrors ReplyReviewDto from erg-backend.
type ReplyReviewRequest struct {
	Reply        string `json:"reply"`
	ReplyContent string `json:"replyContent"`
}

// FeatureReviewRequest mirrors FeatureReviewDto from erg-backend.
type FeatureReviewRequest struct {
	IsFeatured bool `json:"is_featured"`
}

// UpdateReviewStatusRequest mirrors the legacy status update payload.
type UpdateReviewStatusRequest struct {
	Status    string `json:"status" validate:"required"`
	AdminNote string `json:"adminNote,omitempty"`
}

// BulkUpdateStatusRequest mirrors the legacy bulk status payload.
type BulkUpdateStatusRequest struct {
	IDs    []string `json:"ids" validate:"required,min=1"`
	Status string   `json:"status" validate:"required"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

// ReviewResponse is a public review item.
type ReviewResponse struct {
	ID                 string `json:"id"`
	TargetID           string `json:"target_id"`
	TargetType         string `json:"target_type"`
	UserName           string `json:"user_name"`
	UserAvatar         string `json:"user_avatar,omitempty"`
	Rating             int    `json:"rating"`
	Comment            string `json:"comment"`
	IsVerifiedPurchase bool   `json:"is_verified_purchase"`
	IsFeatured         bool   `json:"is_featured"`
	AdminReply         string `json:"admin_reply,omitempty"`
	HelpfulCount       int    `json:"helpful_count"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// ReviewListResponse mirrors NestJS findAllReviews response.
type ReviewListResponse struct {
	Data  []ReviewResponse     `json:"data"`
	Meta  *ListMeta            `json:"meta"`
	Stats *ReviewStatsResponse `json:"stats,omitempty"`
}

// ListMeta contains pagination metadata.
type ListMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	TotalPages int   `json:"totalPages"`
	Limit      int   `json:"limit"`
}

// ReviewStatsResponse holds aggregate rating statistics.
type ReviewStatsResponse struct {
	Average      float64          `json:"average"`
	Count        int64            `json:"count"`
	Distribution map[string]int64 `json:"distribution"`
}

// ReviewQueryParams mirrors ReviewQueryDto from erg-backend.
type ReviewQueryParams struct {
	TargetID   string `query:"targetId"`
	TargetType string `query:"targetType"`
	Limit      int    `query:"limit"`
	Page       int    `query:"page"`
	Sort       string `query:"sort"` // newest | oldest | highest | lowest
}

// AdminReviewListResponse mirrors NestJS admin review findAll response.
type AdminReviewListResponse struct {
	Data []ReviewResponse `json:"data"`
	Meta *ListMeta        `json:"meta"`
}

// AdminReviewQueryParams mirrors admin ReviewQueryDto.
type AdminReviewQueryParams struct {
	Status     string `query:"status"` // pending | approved | rejected
	TargetType string `query:"targetType"`
	TargetID   string `query:"targetId"`
	Limit      int    `query:"limit"`
	Page       int    `query:"page"`
	Sort       string `query:"sort"` // newest | oldest
}

// ─── Conversion Helpers ───────────────────────────────────────────────────────

// ToReviewResponse converts an entity to a public response DTO.
func ToReviewResponse(r *entities.Review) ReviewResponse {
	created := r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	updated := ""
	if !r.UpdatedAt.IsZero() {
		updated = r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return ReviewResponse{
		ID:                 r.ID.Hex(),
		TargetID:           r.TargetID,
		TargetType:         string(r.TargetType),
		UserName:           r.UserName,
		UserAvatar:         r.UserAvatar,
		Rating:             r.Rating,
		Comment:            r.Comment,
		IsVerifiedPurchase: r.IsVerifiedPurchase,
		IsFeatured:         r.IsFeatured,
		AdminReply:         r.AdminReply,
		HelpfulCount:       r.HelpfulCount,
		CreatedAt:          created,
		UpdatedAt:          updated,
	}
}

// ToReviewResponses converts a slice of entities to response DTOs.
func ToReviewResponses(reviews []*entities.Review) []ReviewResponse {
	out := make([]ReviewResponse, len(reviews))
	for i, r := range reviews {
		out[i] = ToReviewResponse(r)
	}
	return out
}

// ─── Validation ───────────────────────────────────────────────────────────────

// Validate validates a request struct using go-playground/validator.
func Validate(v interface{}) error {
	return validator.New().Struct(v)
}
