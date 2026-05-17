// Package entities defines the domain models for the profiles module.
package entities

import "time"

// Profile represents a user's extended public-facing profile.
type Profile struct {
	ID          string     `bson:"_id,omitempty" json:"id"`
	UserID      string     `bson:"user_id" json:"user_id"` // unique, links to auth.users
	FullName    string     `bson:"full_name" json:"fullName"`
	Bio         string     `bson:"bio,omitempty" json:"bio,omitempty"`
	Phone       string     `bson:"phone,omitempty" json:"phone,omitempty"`
	DateOfBirth *time.Time `bson:"date_of_birth,omitempty" json:"date_of_birth,omitempty"`
	Gender      string     `bson:"gender,omitempty" json:"gender,omitempty"`
	Address     string     `bson:"address,omitempty" json:"address,omitempty"`
	City        string     `bson:"city,omitempty" json:"city,omitempty"`
	District    string     `bson:"district,omitempty" json:"district,omitempty"`
	SocialLinks string     `bson:"social_links,omitempty" json:"social_links,omitempty"` // JSON: {linkedin, twitter, facebook}
	AvatarURL   string     `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	IsCompleted bool       `bson:"is_profile_completed" json:"isProfileCompleted"`
	CreatedAt   time.Time  `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// ProfileCollection is the MongoDB collection name for profiles.
const ProfileCollection = "profiles"
