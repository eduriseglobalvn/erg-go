package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuditLog represents an audit trail entry for administrative actions.
type AuditLog struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       string        `bson:"user_id" json:"user_id"`
	UserEmail    string        `bson:"user_email" json:"user_email"`
	Action       string        `bson:"action" json:"action"`               // create, update, delete, restore, upload
	ResourceType string        `bson:"resource_type" json:"resource_type"` // posts, users, seo, courses
	ResourceID   string        `bson:"resource_id" json:"resource_id"`
	Changes      string        `bson:"changes,omitempty" json:"changes,omitempty"` // JSON diff
	IPAddress    string        `bson:"ip_address" json:"ip_address"`
	UserAgent    string        `bson:"user_agent" json:"user_agent"`
	TenantID     string        `bson:"tenant_id" json:"tenant_id"`
	Timestamp    time.Time     `bson:"timestamp" json:"timestamp"`
}

// AuditLogCollection is the MongoDB collection name for audit logs.
const AuditLogCollection = "admin_audit_logs"
