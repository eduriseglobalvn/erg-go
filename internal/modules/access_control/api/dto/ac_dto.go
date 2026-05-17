// Package dto provides request/response types for the access_control module.
package dto

import "time"

// ─── Role DTOs ────────────────────────────────────────────────────────────────

// CreateRoleRequest is the payload for POST /api/access-control/roles.
type CreateRoleRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=100"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// UpdateRoleRequest is the payload for PUT /api/access-control/roles/:id.
type UpdateRoleRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// RoleResponse is the public-facing role document.
type RoleResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Permissions []string  `json:"permissions"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ─── Permission DTOs ──────────────────────────────────────────────────────────

// PermissionResponse is the public-facing permission document.
type PermissionResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
	Label string `json:"label"`
	Desc  string `json:"description,omitempty"`
}

// ─── Permission Group DTOs ───────────────────────────────────────────────────

// PermissionGroupResponse is the public-facing permission group document.
type PermissionGroupResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Label       string               `json:"label"`
	Order       int                  `json:"order"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
}

// ─── User Role Assignment DTOs ─────────────────────────────────────────────

// AssignRolesRequest is the payload for POST /api/access-control/users/:userId/roles.
type AssignRolesRequest struct {
	RoleIDs []string `json:"roleIds" validate:"required,min=1"`
}

// BulkAssignRoleRequest is the payload for POST /api/access-control/bulk-assign-role.
type BulkAssignRoleRequest struct {
	UserIDs []string `json:"userIds" validate:"required,min=1"`
	RoleID  string   `json:"roleId" validate:"required"`
}

// ─── Permission Override DTOs ────────────────────────────────────────────────

// AddPermissionOverrideRequest is the payload for POST /api/access-control/users/:userId/permissions.
type AddPermissionOverrideRequest struct {
	GrantType  string     `json:"grant_type" validate:"required,oneof=GRANT DENY"`
	Permission string     `json:"permission" validate:"required"`
	Reason     string     `json:"reason,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// PermissionOverrideResponse is the public-facing override document.
type PermissionOverrideResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Permission string     `json:"permission"`
	GrantType  string     `json:"grant_type"`
	Reason     string     `json:"reason,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// PreviewChangesRequest is the payload for POST /api/access-control/users/:userId/preview.
type PreviewChangesRequest struct {
	Changes map[string]any `json:"changes"`
}

// EffectivePermissionsResponse combines role-based and override-based permissions.
type EffectivePermissionsResponse struct {
	UserID               string   `json:"user_id"`
	RolePermissions      []string `json:"role_permissions"`
	GrantedPermissions   []string `json:"granted_permissions"`
	DeniedPermissions    []string `json:"denied_permissions"`
	EffectivePermissions []string `json:"effective_permissions"` // final merged set
}

// FeatureConfigResponse holds UI feature flags derived from user permissions.
type FeatureConfigResponse struct {
	UserID        string                `json:"user_id"`
	Permissions   []string              `json:"permissions"`
	Features      map[string]bool       `json:"features"` // legacy boolean flags
	FeatureAccess FeatureAccessResponse `json:"featureAccess,omitempty"`
	Roles         []string              `json:"roles,omitempty"`
}

// FeatureAccessResponse mirrors the erg-backend feature access payload.
type FeatureAccessResponse struct {
	Sidebar          []string `json:"sidebar,omitempty"`
	DashboardWidgets []string `json:"dashboardWidgets,omitempty"`
	QuickActions     []string `json:"quickActions,omitempty"`
}
