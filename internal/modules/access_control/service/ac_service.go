// Package service provides business logic for the access_control module.
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"erg.ninja/internal/modules/access_control/dto"
	"erg.ninja/internal/modules/access_control/entities"
	"erg.ninja/internal/modules/access_control/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Service provides access control business logic.
type Service struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewService creates a new access_control service.
func NewService(gormClient *database.GORMPostgresClient, log *logger.Logger) *Service {
	return &Service{
		repo: repository.NewRepository(gormClient, log),
		log:  log,
	}
}

// Repository returns the underlying repository for use by the controller.
func (s *Service) Repository() *repository.Repository {
	return s.repo
}

// ─── Role Operations ───────────────────────────────────────────────────────

// CreateRole creates a new role.
func (s *Service) CreateRole(ctx context.Context, req dto.CreateRoleRequest) (*entities.Role, error) {
	// Check for duplicate name.
	existing, err := s.repo.GetRoleByName(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("access_control.CreateRole: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("access_control.CreateRole: role name already exists")
	}

	role := &entities.Role{
		ID:          database.NewID(),
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsDefault:   false,
	}
	if err := s.repo.CreateRole(ctx, role); err != nil {
		return nil, fmt.Errorf("access_control.CreateRole: %w", err)
	}
	return role, nil
}

// GetRole returns a role by ID.
func (s *Service) GetRole(ctx context.Context, id string) (*entities.Role, error) {
	role, err := s.repo.GetRoleByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("access_control.GetRole: %w", err)
	}
	return role, nil
}

// ListRoles returns all roles.
func (s *Service) ListRoles(ctx context.Context) ([]*entities.Role, error) {
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("access_control.ListRoles: %w", err)
	}
	return roles, nil
}

// UpdateRole updates an existing role.
func (s *Service) UpdateRole(ctx context.Context, id string, req dto.UpdateRoleRequest) (*entities.Role, error) {
	existing, err := s.repo.GetRoleByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("access_control.UpdateRole.GetRoleByID: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	if req.Name != "" && req.Name != existing.Name {
		// Check for duplicate name.
		conflict, _ := s.repo.GetRoleByName(ctx, req.Name)
		if conflict != nil {
			return nil, fmt.Errorf("access_control.UpdateRole: role name already exists")
		}
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Permissions != nil {
		updates["permissions"] = req.Permissions
	}
	if len(updates) == 0 {
		return existing, nil
	}

	if err := s.repo.UpdateRole(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("access_control.UpdateRole: %w", err)
	}
	return s.repo.GetRoleByID(ctx, id)
}

// DeleteRole deletes a role by ID.
func (s *Service) DeleteRole(ctx context.Context, id string) error {
	role, err := s.repo.GetRoleByID(ctx, id)
	if err != nil {
		return fmt.Errorf("access_control.DeleteRole: %w", err)
	}
	if role == nil {
		return fmt.Errorf("access_control.DeleteRole: role not found")
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		return fmt.Errorf("access_control.DeleteRole: %w", err)
	}
	return nil
}

// ─── Permission Operations ──────────────────────────────────────────────────

// ListPermissions returns all permissions.
func (s *Service) ListPermissions(ctx context.Context) ([]*entities.Permission, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("access_control.ListPermissions: %w", err)
	}
	return perms, nil
}

// ListPermissionGroups returns all permission groups with their permissions.
func (s *Service) ListPermissionGroups(ctx context.Context) ([]*entities.PermissionGroup, error) {
	groups, err := s.repo.ListPermissionGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("access_control.ListPermissionGroups: %w", err)
	}
	return groups, nil
}

// ─── User Role Assignment ───────────────────────────────────────────────────

// AssignRoles replaces a user's stored role names using incoming role IDs or names.
func (s *Service) AssignRoles(ctx context.Context, userID string, roleIDs []string) error {
	normalized, err := s.normalizeRoleRefs(ctx, roleIDs)
	if err != nil {
		return fmt.Errorf("access_control.AssignRoles.normalizeRoleRefs: %w", err)
	}
	return s.saveUserRoles(ctx, userID, normalized)
}

// RemoveRole removes a role from a user.
func (s *Service) RemoveRole(ctx context.Context, userID, roleID string) error {
	existing, err := s.loadUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("access_control.RemoveRole.loadUserRoles: %w", err)
	}
	target, err := s.normalizeSingleRoleRef(ctx, roleID)
	if err != nil {
		return fmt.Errorf("access_control.RemoveRole.normalizeSingleRoleRef: %w", err)
	}
	filtered := make([]string, 0, len(existing))
	for _, r := range existing {
		if r != target {
			filtered = append(filtered, r)
		}
	}
	return s.saveUserRoles(ctx, userID, filtered)
}

// BulkAssignRole assigns a single role to multiple users.
func (s *Service) BulkAssignRole(ctx context.Context, userIDs []string, roleID string) error {
	normalizedRole, err := s.normalizeSingleRoleRef(ctx, roleID)
	if err != nil {
		return fmt.Errorf("access_control.BulkAssignRole.normalizeSingleRoleRef: %w", err)
	}
	for _, uid := range userIDs {
		existing, err := s.loadUserRoles(ctx, uid)
		if err != nil {
			s.log.WarnContext(ctx).Err(err).Str("user_id", uid).Str("role_id", normalizedRole).
				Msg("access_control.BulkAssignRole: failed to load user roles")
			continue
		}
		merged := append(existing, normalizedRole)
		if err := s.saveUserRoles(ctx, uid, uniqueRoleNames(merged)); err != nil {
			s.log.WarnContext(ctx).Err(err).Str("user_id", uid).Str("role_id", roleID).
				Msg("access_control.BulkAssignRole: failed for user")
		}
	}
	return nil
}

// GetUserRoles returns the list of canonical role names for a user.
func (s *Service) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	return s.loadUserRoles(ctx, userID)
}

// ─── Effective Permissions ──────────────────────────────────────────────────

// GetEffectivePermissions computes the final permission set for a user.
// It merges role-based permissions with user-specific overrides (GRANT/DENY).
func (s *Service) GetEffectivePermissions(ctx context.Context, userID string) (*dto.EffectivePermissionsResponse, error) {
	// Gather role-based permissions from canonical role names or legacy role IDs.
	roleRefs, err := s.loadUserRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("access_control.GetEffectivePermissions.loadUserRoles: %w", err)
	}

	rolePerms := make(map[string]bool)
	for _, roleRef := range roleRefs {
		role, err := s.resolveRoleRef(ctx, roleRef)
		if err != nil {
			s.log.WarnContext(ctx).Err(err).Str("role_ref", roleRef).Str("user_id", userID).
				Msg("access_control.GetEffectivePermissions: failed to resolve role reference")
			continue
		}
		if role == nil {
			continue
		}
		for _, p := range role.Permissions {
			rolePerms[p] = true
		}
	}
	var rolePermList []string
	for p := range rolePerms {
		rolePermList = append(rolePermList, p)
	}
	sort.Strings(rolePermList)

	// Gather overrides.
	overrides, err := s.repo.ListOverridesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("access_control.GetEffectivePermissions.ListOverridesByUser: %w", err)
	}

	// Apply overrides (DENY removes, GRANT adds).
	effectiveSet := make(map[string]bool)
	for p := range rolePerms {
		effectiveSet[p] = true
	}

	var granted []string
	var denied []string
	now := time.Now()
	for _, o := range overrides {
		// Skip expired overrides.
		if o.ExpiresAt != nil && o.ExpiresAt.Before(now) {
			continue
		}
		if o.GrantType == "DENY" {
			delete(effectiveSet, o.Permission)
			denied = append(denied, o.Permission)
		} else if o.GrantType == "GRANT" {
			effectiveSet[o.Permission] = true
			granted = append(granted, o.Permission)
		}
	}

	var effective []string
	for p := range effectiveSet {
		effective = append(effective, p)
	}
	sort.Strings(granted)
	sort.Strings(denied)
	sort.Strings(effective)

	return &dto.EffectivePermissionsResponse{
		UserID:               userID,
		RolePermissions:      rolePermList,
		GrantedPermissions:   granted,
		DeniedPermissions:    denied,
		EffectivePermissions: effective,
	}, nil
}

// GetFeatureConfig returns user permissions and derived UI feature flags.
func (s *Service) GetFeatureConfig(ctx context.Context, userID string) (*dto.FeatureConfigResponse, error) {
	perms, err := s.GetEffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool)
	for _, p := range perms.EffectivePermissions {
		permSet[p] = true
	}

	// Derive features from permissions.
	features := map[string]bool{
		"can_manage_users":    permSet["users.read"],
		"can_create_posts":    permSet["posts.create"],
		"can_manage_roles":    permSet["roles.read"],
		"can_view_audit_logs": permSet["system.logs"],
		"can_manage_settings": permSet["system.settings"],
	}

	roleNames, _ := s.loadUserRoles(ctx, userID)
	featureAccess := deriveFeatureAccess(permSet)

	return &dto.FeatureConfigResponse{
		UserID:        userID,
		Permissions:   perms.EffectivePermissions,
		Features:      features,
		FeatureAccess: featureAccess,
		Roles:         roleNames,
	}, nil
}

// ─── Permission Overrides ───────────────────────────────────────────────────

// AddPermissionOverride grants or denies a specific permission to a user.
func (s *Service) AddPermissionOverride(ctx context.Context, userID string, req dto.AddPermissionOverrideRequest, createdBy string) (*entities.UserPermissionOverride, error) {
	override := &entities.UserPermissionOverride{
		ID:         database.NewID(),
		UserID:     userID,
		Permission: req.Permission,
		GrantType:  req.GrantType,
		Reason:     req.Reason,
		ExpiresAt:  req.ExpiresAt,
		CreatedBy:  createdBy,
	}
	if err := s.repo.CreateOverride(ctx, override); err != nil {
		return nil, fmt.Errorf("access_control.AddPermissionOverride: %w", err)
	}
	return override, nil
}

// ListPermissionOverrides returns all overrides for a user.
func (s *Service) ListPermissionOverrides(ctx context.Context, userID string) ([]*entities.UserPermissionOverride, error) {
	overrides, err := s.repo.ListOverridesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("access_control.ListPermissionOverrides: %w", err)
	}
	return overrides, nil
}

// DeletePermissionOverride deletes a specific override.
func (s *Service) DeletePermissionOverride(ctx context.Context, overrideID string) error {
	override, err := s.repo.GetOverrideByID(ctx, overrideID)
	if err != nil {
		return fmt.Errorf("access_control.DeletePermissionOverride: %w", err)
	}
	if override == nil {
		return fmt.Errorf("access_control.DeletePermissionOverride: override not found")
	}
	return s.repo.DeleteOverride(ctx, overrideID)
}

// PreviewChanges simulates the effect of proposed changes on a user's permissions.
func (s *Service) PreviewChanges(ctx context.Context, userID string, changes map[string]any) (map[string]any, error) {
	current, err := s.GetEffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"current_permissions": current.EffectivePermissions,
		"proposed_changes":    changes,
	}

	// Simulate add_roles.
	if addRoles, ok := changes["add_roles"].([]any); ok {
		var roleIDs []string
		for _, r := range addRoles {
			if s, ok := r.(string); ok {
				roleIDs = append(roleIDs, s)
			}
		}
		if len(roleIDs) > 0 {
			result["simulated_add_roles"] = roleIDs
		}
	}

	return result, nil
}

// SeedDefaultData seeds default roles, permissions, and groups on startup.
func (s *Service) SeedDefaultData(ctx context.Context) error {
	return s.repo.SeedDefaultData(ctx)
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// loadUserRoles reads canonical role names from the auth_users document.
func (s *Service) loadUserRoles(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetUserRoles(ctx, userID)
}

// saveUserRoles writes canonical role names to the auth_users document.
func (s *Service) saveUserRoles(ctx context.Context, userID string, roles []string) error {
	return s.repo.SetUserRoles(ctx, userID, uniqueRoleNames(roles))
}

func (s *Service) resolveRoleRef(ctx context.Context, roleRef string) (*entities.Role, error) {
	roleRef = strings.TrimSpace(roleRef)
	if roleRef == "" {
		return nil, nil
	}

	role, err := s.repo.GetRoleByID(ctx, roleRef)
	if err != nil {
		return nil, err
	}
	if role != nil {
		return role, nil
	}
	return s.repo.GetRoleByName(ctx, roleRef)
}

func (s *Service) normalizeSingleRoleRef(ctx context.Context, roleRef string) (string, error) {
	role, err := s.resolveRoleRef(ctx, roleRef)
	if err != nil {
		return "", err
	}
	if role == nil {
		return "", fmt.Errorf("role %q not found", roleRef)
	}
	return role.Name, nil
}

func (s *Service) normalizeRoleRefs(ctx context.Context, roleRefs []string) ([]string, error) {
	result := make([]string, 0, len(roleRefs))
	for _, roleRef := range roleRefs {
		name, err := s.normalizeSingleRoleRef(ctx, roleRef)
		if err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return uniqueRoleNames(result), nil
}

func uniqueRoleNames(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func deriveFeatureAccess(permSet map[string]bool) dto.FeatureAccessResponse {
	type featureModule struct {
		prefixes         []string
		sidebar          []string
		dashboardWidgets []string
		quickActions     []string
	}

	modules := []featureModule{
		{
			prefixes:         []string{"posts.", "courses.", "crawler.", "reviews."},
			sidebar:          []string{"posts", "courses", "crawler", "reviews"},
			dashboardWidgets: []string{"post-stats", "review-stats"},
			quickActions:     []string{"create-post", "run-crawler"},
		},
		{
			prefixes:         []string{"seo."},
			sidebar:          []string{"seo-dashboard", "seo-settings"},
			dashboardWidgets: []string{"seo-overview"},
			quickActions:     []string{"analyze-seo"},
		},
		{
			prefixes:         []string{"users.", "roles."},
			sidebar:          []string{"users", "roles"},
			dashboardWidgets: []string{"user-stats"},
			quickActions:     []string{"create-user"},
		},
		{
			prefixes:         []string{"system.", "api-keys.", "audit.", "menus.", "pages."},
			sidebar:          []string{"settings", "api-keys", "audit-logs", "menus", "pages"},
			dashboardWidgets: []string{"system-health"},
			quickActions:     []string{"clear-cache"},
		},
		{
			prefixes:         []string{"recruitment."},
			sidebar:          []string{"jobs", "candidates"},
			dashboardWidgets: []string{"recruitment-stats"},
			quickActions:     []string{"create-job"},
		},
		{
			prefixes:         []string{"analytics."},
			sidebar:          []string{"analytics"},
			dashboardWidgets: []string{"traffic-overview"},
			quickActions:     []string{"export-report"},
		},
	}

	isAdmin := permSet["system.settings"]
	var sidebar []string
	var dashboardWidgets []string
	var quickActions []string

	for _, module := range modules {
		if !isAdmin && !hasAnyPermissionPrefix(permSet, module.prefixes) {
			continue
		}
		sidebar = appendUniqueStrings(sidebar, module.sidebar...)
		dashboardWidgets = appendUniqueStrings(dashboardWidgets, module.dashboardWidgets...)
		quickActions = appendUniqueStrings(quickActions, module.quickActions...)
	}

	return dto.FeatureAccessResponse{
		Sidebar:          sidebar,
		DashboardWidgets: dashboardWidgets,
		QuickActions:     quickActions,
	}
}

func hasAnyPermissionPrefix(permSet map[string]bool, prefixes []string) bool {
	for perm := range permSet {
		for _, prefix := range prefixes {
			if strings.HasPrefix(perm, prefix) {
				return true
			}
		}
	}
	return false
}

func appendUniqueStrings(dst []string, values ...string) []string {
	if len(values) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}
