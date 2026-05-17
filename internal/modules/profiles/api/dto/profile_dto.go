// Package dto provides request/response types for the profiles module.
package dto

import "time"

// ─── Request DTOs ────────────────────────────────────────────────────────────

// CreateProfileRequest is the payload for POST /api/profiles.
type CreateProfileRequest struct {
	UserID      string     `json:"user_id" validate:"required"`
	FullName    string     `json:"fullName" validate:"required,min=1,max=200"`
	Bio         string     `json:"bio,omitempty"`
	Phone       string     `json:"phone,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `json:"gender,omitempty"`
	Address     string     `json:"address,omitempty"`
	City        string     `json:"city,omitempty"`
	District    string     `json:"district,omitempty"`
	SocialLinks string     `json:"social_links,omitempty"` // JSON string: {linkedin, twitter, facebook}
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// UpdateProfileRequest is the payload for PUT /api/profiles/:userId.
type UpdateProfileRequest struct {
	FullName    string     `json:"full_name,omitempty"`
	Bio         string     `json:"bio,omitempty"`
	Phone       string     `json:"phone,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `json:"gender,omitempty"`
	Address     string     `json:"address,omitempty"`
	City        string     `json:"city,omitempty"`
	District    string     `json:"district,omitempty"`
	SocialLinks string     `json:"social_links,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

// ProfileResponse is the public-facing profile document.
type ProfileResponse struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	FullName    string     `json:"fullName"`
	Bio         string     `json:"bio,omitempty"`
	Phone       string     `json:"phone,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `json:"gender,omitempty"`
	Address     string     `json:"address,omitempty"`
	City        string     `json:"city,omitempty"`
	District    string     `json:"district,omitempty"`
	SocialLinks string     `json:"social_links,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	IsCompleted bool       `json:"isProfileCompleted"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
}

// SocialLinksResponse parses and returns the social links as a structured object.
type SocialLinksResponse struct {
	LinkedIn string `json:"linkedin,omitempty"`
	Twitter  string `json:"twitter,omitempty"`
	Facebook string `json:"facebook,omitempty"`
}
