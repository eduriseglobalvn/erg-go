package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserSession represents a user login session stored in MongoDB.
type UserSession struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID        bson.ObjectID `bson:"user_id" json:"user_id"`
	SessionID     string        `bson:"session_id" json:"session_id"`
	DeviceID      string        `bson:"device_id" json:"device_id"`
	DeviceName    string        `bson:"device_name" json:"device_name"`
	IPAddress     string        `bson:"ip_address" json:"ip_address"`
	UserAgent     string        `bson:"user_agent" json:"user_agent"`
	RefreshToken  string        `bson:"refresh_token_hash" json:"-"` // hashed, never expose
	TenantID      string        `bson:"tenant_id" json:"tenant_id"`
	ExpiresAt     time.Time     `bson:"expires_at" json:"expires_at"`
	RevokedAt     *time.Time    `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
	RevokedReason string        `bson:"revoked_reason,omitempty" json:"revoked_reason,omitempty"`
	LastActiveAt  time.Time     `bson:"last_active_at" json:"last_active_at"`
	CreatedAt     time.Time     `bson:"created_at" json:"createdAt"`
}

// IsRevoked returns true if the session has been revoked.
func (s *UserSession) IsRevoked() bool {
	return s.RevokedAt != nil
}
