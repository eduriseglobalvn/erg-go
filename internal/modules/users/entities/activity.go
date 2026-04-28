package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ActivityType categorises the type of user activity.
type ActivityType string

const (
	ActivityLogin          ActivityType = "LOGIN"
	ActivityLogout         ActivityType = "LOGOUT"
	ActivityProfileUpdate  ActivityType = "PROFILE_UPDATE"
	ActivityPasswordChange ActivityType = "PASSWORD_CHANGE"
	ActivityStatusChange   ActivityType = "STATUS_CHANGE"
	ActivityRoleAssign     ActivityType = "ROLE_ASSIGN"
	ActivityRoleRevoke     ActivityType = "ROLE_REVOKE"
	ActivitySessionRevoke  ActivityType = "SESSION_REVOKE"
	ActivityOnboarding     ActivityType = "ONBOARDING"
	ActivityAdminCreate    ActivityType = "ADMIN_CREATE"
	ActivityAdminDelete    ActivityType = "ADMIN_DELETE"
	ActivityBulkStatus     ActivityType = "BULK_STATUS"
	ActivityBulkDelete     ActivityType = "BULK_DELETE"
)

// UserActivity logs a user action into MongoDB for audit/compliance.
// Stored in the "user_activities" MongoDB collection.
type UserActivity struct {
	ID           bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID       bson.ObjectID  `bson:"user_id" json:"user_id"`
	TenantID     string         `bson:"tenant_id" json:"tenant_id"`
	Action       ActivityType   `bson:"action" json:"action"`
	TargetUserID *bson.ObjectID `bson:"target_user_id,omitempty" json:"target_user_id,omitempty"`
	IPAddress    string         `bson:"ip_address" json:"ip_address"`
	UserAgent    string         `bson:"user_agent" json:"user_agent"`
	Description  string         `bson:"description" json:"description"`
	// Metadata holds arbitrary context (old/new values, affected fields, etc.).
	Metadata  map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt time.Time      `bson:"created_at" json:"createdAt"`
}
