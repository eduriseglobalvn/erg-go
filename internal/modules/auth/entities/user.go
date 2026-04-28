package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserStatus represents the account status of a user.
type UserStatus string

const (
	UserStatusActive  UserStatus = "ACTIVE"
	UserStatusPending UserStatus = "PENDING"
	UserStatusBanned  UserStatus = "BANNED"
	UserStatusBlocked UserStatus = "BLOCKED"
)

// User represents a user entity stored in MongoDB.
// Extends erg-backend User entity fields to cover full profile.
type User struct {
	ID                  bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email               string        `bson:"email" json:"email"`
	PasswordHash        string        `bson:"password_hash" json:"-"`
	FullName            string        `bson:"full_name" json:"fullName"`
	AvatarURL           string        `bson:"avatar_url" json:"avatarUrl"`
	Status              UserStatus    `bson:"status" json:"status"`
	Provider            string        `bson:"provider" json:"provider"` // "local", "google", "facebook", "apple"
	ProviderID          string        `bson:"provider_id" json:"provider_id"`
	AccountType         string        `bson:"account_type,omitempty" json:"accountType,omitempty"` // "erg", "google", "hybrid"
	GoogleSub           string        `bson:"google_sub,omitempty" json:"googleSub,omitempty"`
	GoogleEmail         string        `bson:"google_email,omitempty" json:"googleEmail,omitempty"`
	GoogleEmailVerified bool          `bson:"google_email_verified,omitempty" json:"googleEmailVerified,omitempty"`
	LastLoginProvider   string        `bson:"last_login_provider,omitempty" json:"lastLoginProvider,omitempty"`
	Roles               []string      `bson:"roles" json:"roles"`
	TenantID            string        `bson:"tenant_id" json:"tenant_id"`
	// Extended profile fields (mirrors erg-backend User entity):
	Phone              string            `bson:"phone,omitempty" json:"phone,omitempty"`
	Bio                string            `bson:"bio,omitempty" json:"bio,omitempty"`
	Gender             string            `bson:"gender,omitempty" json:"gender,omitempty"`
	DateOfBirth        string            `bson:"date_of_birth,omitempty" json:"date_of_birth,omitempty"`
	Address            string            `bson:"address,omitempty" json:"address,omitempty"`
	City               string            `bson:"city,omitempty" json:"city,omitempty"`
	District           string            `bson:"district,omitempty" json:"district,omitempty"`
	JobTitle           string            `bson:"job_title,omitempty" json:"job_title,omitempty"`
	Region             string            `bson:"region,omitempty" json:"region,omitempty"`
	SocialLinks        map[string]string `bson:"social_links,omitempty" json:"social_links,omitempty"`
	ExtendedProfile    string            `bson:"extended_profile,omitempty" json:"-"` // JSON blob for future extensibility
	IsProfileCompleted bool              `bson:"is_profile_completed,omitempty" json:"isProfileCompleted"`
	LastLoginAt        *time.Time        `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	LoginCount         int64             `bson:"login_count,omitempty" json:"login_count,omitempty"`
	CreatedAt          time.Time         `bson:"created_at" json:"createdAt"`
	UpdatedAt          time.Time         `bson:"updated_at" json:"updatedAt"`
}
