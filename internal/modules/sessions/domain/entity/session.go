// Package entities defines domain models for the sessions module.
package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Session represents a user session stored in MongoDB auth_sessions collection.
type Session struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	TenantID   string        `bson:"tenant_id"`
	UserID     bson.ObjectID `bson:"user_id"`
	SessionID  string        `bson:"session_id"`
	IPAddress  string        `bson:"ip_address"`
	UserAgent  string        `bson:"user_agent"`
	CreatedAt  time.Time     `bson:"created_at"`
	LastActive time.Time     `bson:"last_active"`
	ExpiresAt  time.Time     `bson:"expires_at"`
	IsActive   bool          `bson:"is_active"`
	RevokedAt  *time.Time    `bson:"revoked_at,omitempty"`
}

// User represents a user stored in MongoDB auth_users collection.
type User struct {
	ID                 bson.ObjectID `bson:"_id,omitempty"`
	TenantID           string        `bson:"tenant_id"`
	Email              string        `bson:"email"`
	FullName           string        `bson:"full_name"`
	AvatarURL          string        `bson:"avatar_url,omitempty"`
	Provider           string        `bson:"provider,omitempty"`
	AccountType        string        `bson:"account_type,omitempty"`
	LastLoginProvider  string        `bson:"last_login_provider,omitempty"`
	Status             string        `bson:"status"`
	Roles              []string      `bson:"roles,omitempty"`
	IsProfileCompleted bool          `bson:"is_profile_completed"`
}

// UserStatus constants (mirrors auth module entities).
const (
	UserStatusPending = "PENDING"
	UserStatusActive  = "ACTIVE"
	UserStatusBanned  = "BANNED"
	UserStatusBlocked = "BLOCKED"
)
