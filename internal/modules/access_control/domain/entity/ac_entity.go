// Package entities defines the domain models for the access_control module.
package entities

import "time"

// Role represents a named collection of permissions in the system.
type Role struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	Name        string    `bson:"name" json:"name"`
	Description string    `bson:"description,omitempty" json:"description,omitempty"`
	Permissions []string  `bson:"permissions" json:"permissions"` // e.g. "users.read", "posts.create"
	IsDefault   bool      `bson:"is_default" json:"is_default"`
	CreatedAt   time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// RoleCollection is the MongoDB collection name for roles.
const RoleCollection = "ac_roles"

// Permission represents a single granular permission.
type Permission struct {
	ID    string `bson:"_id,omitempty" json:"id"`
	Name  string `bson:"name" json:"name"`   // e.g. "users.read"
	Group string `bson:"group" json:"group"` // e.g. "users", "posts", "roles", "system"
	Label string `bson:"label" json:"label"`
	Desc  string `bson:"description,omitempty" json:"description,omitempty"`
}

// PermissionCollection is the MongoDB collection name for permissions.
const PermissionCollection = "ac_permissions"

// PermissionGroup groups related permissions for display.
type PermissionGroup struct {
	ID    string `bson:"_id,omitempty" json:"id"`
	Name  string `bson:"name" json:"name"`
	Label string `bson:"label" json:"label"`
	Order int    `bson:"order" json:"order"`
}

// PermissionGroupCollection is the MongoDB collection name for permission groups.
const PermissionGroupCollection = "ac_permission_groups"

// UserPermissionOverride grants or denies a specific permission to a user.
type UserPermissionOverride struct {
	ID         string     `bson:"_id,omitempty" json:"id"`
	UserID     string     `bson:"user_id" json:"user_id"`
	Permission string     `bson:"permission" json:"permission"`
	GrantType  string     `bson:"grant_type" json:"grant_type"` // GRANT | DENY
	Reason     string     `bson:"reason,omitempty" json:"reason,omitempty"`
	ExpiresAt  *time.Time `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	CreatedBy  string     `bson:"created_by" json:"created_by"`
	CreatedAt  time.Time  `bson:"created_at" json:"createdAt"`
}

// UserPermissionOverrideCollection is the MongoDB collection name.
const UserPermissionOverrideCollection = "ac_user_permission_overrides"
