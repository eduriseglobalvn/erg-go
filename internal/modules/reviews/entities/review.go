package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReviewStatus represents the moderation status of a review.
type ReviewStatus string

// ReviewTargetType represents what kind of entity is being reviewed.
type ReviewTargetType string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
)

const (
	ReviewTargetTypePost    ReviewTargetType = "post"
	ReviewTargetTypeCourse  ReviewTargetType = "course"
	ReviewTargetTypeProduct ReviewTargetType = "product"
)

// Review represents a user review for a post, course, or product.
type Review struct {
	ID                 bson.ObjectID    `bson:"_id,omitempty" json:"id"`
	TargetID           string           `bson:"target_id" json:"target_id"`
	TargetType         ReviewTargetType `bson:"target_type" json:"target_type"`
	UserID             bson.ObjectID    `bson:"user_id,omitempty" json:"user_id,omitempty"`
	UserName           string           `bson:"user_name" json:"user_name"`
	UserEmail          string           `bson:"user_email,omitempty" json:"user_email,omitempty"`
	UserAvatar         string           `bson:"user_avatar,omitempty" json:"user_avatar,omitempty"`
	Rating             int              `bson:"rating" json:"rating"`
	Comment            string           `bson:"comment" json:"comment"`
	IsVerifiedPurchase bool             `bson:"is_verified_purchase" json:"is_verified_purchase"`
	Status             ReviewStatus     `bson:"status" json:"status"`
	AdminReply         string           `bson:"admin_reply,omitempty" json:"admin_reply,omitempty"`
	ReplyBy            string           `bson:"reply_by,omitempty" json:"reply_by,omitempty"`
	ReplyAt            *time.Time       `bson:"reply_at,omitempty" json:"reply_at,omitempty"`
	AdminNote          string           `bson:"admin_note,omitempty" json:"admin_note,omitempty"`
	ReviewedBy         string           `bson:"reviewed_by,omitempty" json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time       `bson:"reviewed_at,omitempty" json:"reviewed_at,omitempty"`
	IsFeatured         bool             `bson:"is_featured" json:"is_featured"`
	HelpfulCount       int              `bson:"helpful_count" json:"helpful_count"`
	IPAddress          string           `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	UserAgent          string           `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	CreatedAt          time.Time        `bson:"created_at" json:"createdAt"`
	UpdatedAt          time.Time        `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}

const ReviewCollection = "reviews"
