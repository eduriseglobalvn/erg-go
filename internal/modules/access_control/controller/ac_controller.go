// Package controller handles HTTP requests for the access_control module.
package controller

import (
	"context"
	"fmt"

	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/access_control/dto"
	"erg.ninja/internal/modules/access_control/entities"
	"erg.ninja/internal/modules/access_control/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for access control.
type Controller struct {
	svc          *service.Service
	jwtValidator *auth.JWTValidator
	log          *logger.Logger
}

// NewController creates a new access_control controller.
func NewController(svc *service.Service, jwtValidator *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, jwtValidator: jwtValidator, log: log}
}

// RegisterRoutes mounts the access control REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/access-control")
	api.Use(c.authMiddleware())
	api.Use(c.requirePermission("roles.read"))

	api.GET("/roles", c.ListRoles)
	api.GET("/roles/:id", c.GetRole)
	api.GET("/permissions", c.ListPermissions)
	api.GET("/permission-groups", c.ListPermissionGroups)

	// Role management (create)
	roleCreate := api.Group("")
	roleCreate.Use(c.requirePermission("roles.create"))
	roleCreate.POST("/roles", c.CreateRole)

	// Role management (update)
	roleUpdate := api.Group("")
	roleUpdate.Use(c.requirePermission("roles.update"))
	roleUpdate.PUT("/roles/:id", c.UpdateRole)
	roleUpdate.PATCH("/roles/:id", c.UpdateRole)

	// Role management (delete)
	roleDelete := api.Group("")
	roleDelete.Use(c.requirePermission("roles.delete"))
	roleDelete.DELETE("/roles/:id", c.DeleteRole)

	api.GET("/feature-config", c.GetFeatureConfig)

	// User role assignment
	userMgmt := api.Group("")
	userMgmt.Use(c.requirePermission("roles.assign"))
	userMgmt.POST("/users/:userId/roles", c.AssignRoles)
	userMgmt.PATCH("/users/:userId/roles", c.AssignRoles)
	userMgmt.DELETE("/users/:userId/roles/:roleId", c.RemoveRole)
	userMgmt.POST("/users/:userId/permissions", c.AddPermissionOverride)
	userMgmt.DELETE("/users/:userId/permissions/:overrideId", c.DeletePermissionOverride)
	userMgmt.POST("/bulk-assign-role", c.BulkAssignRole)

	// User permission read
	userRead := api.Group("")
	userRead.Use(c.requirePermission("users.read"), c.requirePermission("roles.read"))
	userRead.GET("/users/:userId/effective-permissions", c.GetEffectivePermissions)
	userRead.GET("/users/:userId/permissions", c.GetEffectivePermissions)
	userRead.GET("/users/:userId/permission-overrides", c.ListPermissionOverrides)

	api.POST("/users/:userId/preview", c.PreviewChanges)
}

// ─── Auth Middleware ──────────────────────────────────────────────────────────

func (c *Controller) authMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.jwtValidator == nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		claims, err := c.jwtValidator.ValidateRequest(ctx.GetHeader("Authorization"))
		if err != nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		newCtx := contextWithClaims(ctx.Request.Context(), claims)
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

func (c *Controller) requirePermission(perm string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims := getClaims(ctx.Request.Context())
		if claims == nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		if !hasPermission(claims, perm) {
			c.log.WarnContext(ctx.Request.Context()).Str("permission", perm).Str("user_id", claims.UserID).
				Msg("access_control: permission denied")
			response.ForbiddenGin(ctx)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func hasPermission(claims *auth.JWTClaims, required string) bool {
	for _, role := range claims.Roles {
		if strings.EqualFold(role, "admin") {
			return true
		}
	}
	for _, p := range claims.Permissions {
		if p == required {
			return true
		}
	}
	return false
}

// contextKey is a custom type for context keys.
type contextKey string

const claimsCtxKey contextKey = "jwt_claims"

func contextWithClaims(ctx context.Context, claims *auth.JWTClaims) context.Context {
	return context.WithValue(ctx, claimsCtxKey, claims)
}

func getClaims(ctx context.Context) *auth.JWTClaims {
	if v := ctx.Value(claimsCtxKey); v != nil {
		return v.(*auth.JWTClaims)
	}
	return nil
}

func getUserIDFromCtx(ctx context.Context) string {
	if c := getClaims(ctx); c != nil {
		return c.UserID
	}
	return ""
}

func page(ctx *gin.Context, key string, fallback int) int {
	v := ctx.Query(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func joinComma(parts []string) string {
	return strings.Join(parts, ", ")
}

func toRoleResponse(role *entities.Role) dto.RoleResponse {
	if role == nil {
		return dto.RoleResponse{}
	}
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	return dto.RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		Permissions: perms,
		IsDefault:   role.IsDefault,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func toOverrideResponse(o *entities.UserPermissionOverride) dto.PermissionOverrideResponse {
	if o == nil {
		return dto.PermissionOverrideResponse{}
	}
	return dto.PermissionOverrideResponse{
		ID: o.ID, UserID: o.UserID, Permission: o.Permission,
		GrantType: o.GrantType, Reason: o.Reason,
		ExpiresAt: o.ExpiresAt, CreatedBy: o.CreatedBy, CreatedAt: o.CreatedAt,
	}
}

// ListRoles handles GET /api/access-control/roles.
// @Summary List all roles
// @Description Returns all available roles.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/access-control/roles [get]
func (c *Controller) ListRoles(ctx *gin.Context) {
	roles, err := c.svc.ListRoles(ctx.Request.Context())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("access_control: ListRoles failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	items := make([]dto.RoleResponse, len(roles))
	for i, role := range roles {
		items[i] = toRoleResponse(role)
	}
	response.SuccessGin(ctx, map[string]any{"data": items, "items": items, "total": len(items)})
}

// GetRole handles GET /api/access-control/roles/:id.
// @Summary Get a role
// @Description Returns a role by ID.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/roles/{id} [get]
func (c *Controller) GetRole(ctx *gin.Context) {
	role, err := c.svc.GetRole(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", ctx.Param("id")).Msg("access_control: GetRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if role == nil {
		response.NotFoundGin(ctx, "role not found")
		return
	}
	response.SuccessGin(ctx, toRoleResponse(role))
}

// CreateRole handles POST /api/access-control/roles.
// @Summary Create a role
// @Description Creates a new role with permissions.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/access-control/roles [post]
func (c *Controller) CreateRole(ctx *gin.Context) {
	var req dto.CreateRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	if req.Name == "" {
		response.BadRequestGin(ctx, fmt.Errorf("name is required"))
		return
	}
	role, err := c.svc.CreateRole(ctx.Request.Context(), req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("access_control: CreateRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, toRoleResponse(role))
}

// UpdateRole handles PUT /api/access-control/roles/:id.
// @Summary Update a role
// @Description Updates an existing role.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/roles/{id} [put]
func (c *Controller) UpdateRole(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}
	var req dto.UpdateRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	role, err := c.svc.UpdateRole(ctx.Request.Context(), id, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("access_control: UpdateRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if role == nil {
		response.NotFoundGin(ctx, "role not found")
		return
	}
	response.SuccessGin(ctx, toRoleResponse(role))
}

// DeleteRole handles DELETE /api/access-control/roles/:id.
// @Summary Delete a role
// @Description Deletes a role by ID.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/roles/{id} [delete]
func (c *Controller) DeleteRole(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}
	if err := c.svc.DeleteRole(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("access_control: DeleteRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "role deleted"})
}

// ListPermissions handles GET /api/access-control/permissions.
// @Summary List all permissions
// @Description Returns all system permissions.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/access-control/permissions [get]
func (c *Controller) ListPermissions(ctx *gin.Context) {
	perms, err := c.svc.ListPermissions(ctx.Request.Context())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("access_control: ListPermissions failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	items := make([]dto.PermissionResponse, len(perms))
	for i, p := range perms {
		items[i] = dto.PermissionResponse{ID: p.ID, Name: p.Name, Group: p.Group, Label: p.Label, Desc: p.Desc}
	}
	response.SuccessGin(ctx, map[string]any{"data": items, "items": items, "total": len(items)})
}

// ListPermissionGroups handles GET /api/access-control/permission-groups.
// @Summary List permission groups
// @Description Returns permission groups for categorization.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/access-control/permission-groups [get]
func (c *Controller) ListPermissionGroups(ctx *gin.Context) {
	groups, err := c.svc.ListPermissionGroups(ctx.Request.Context())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("access_control: ListPermissionGroups failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	items := make([]dto.PermissionGroupResponse, len(groups))
	for i, g := range groups {
		items[i] = dto.PermissionGroupResponse{ID: g.ID, Name: g.Name, Label: g.Label, Order: g.Order}
	}
	response.SuccessGin(ctx, map[string]any{"data": items, "items": items, "total": len(items)})
}

// GetFeatureConfig handles GET /api/access-control/feature-config.
// @Summary Get feature configuration
// @Description Returns feature flags/config for access control.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/access-control/feature-config [get]
func (c *Controller) GetFeatureConfig(ctx *gin.Context) {
	userID := getUserIDFromCtx(ctx.Request.Context())
	if userID == "" {
		response.UnauthorizedGin(ctx)
		return
	}
	cfg, err := c.svc.GetFeatureConfig(ctx.Request.Context(), userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: GetFeatureConfig failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, cfg)
}

// AssignRoles handles POST /api/access-control/users/:userId/roles.
// @Summary Assign roles to user
// @Description Assigns one or more roles to a specific user.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/roles [post]
func (c *Controller) AssignRoles(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	var req dto.AssignRolesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	if len(req.RoleIDs) == 0 {
		response.BadRequestGin(ctx, fmt.Errorf("roleIds is required"))
		return
	}
	if err := c.svc.AssignRoles(ctx.Request.Context(), userID, req.RoleIDs); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: AssignRoles failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "roles assigned successfully"})
}

// RemoveRole handles DELETE /api/access-control/users/:userId/roles/:roleId.
// @Summary Remove role from user
// @Description Removes a specific role from a user.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Param roleId path string true "Role ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/roles/{roleId} [delete]
func (c *Controller) RemoveRole(ctx *gin.Context) {
	userID := ctx.Param("userId")
	roleID := ctx.Param("roleId")
	if userID == "" || roleID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId and roleId are required"))
		return
	}
	if err := c.svc.RemoveRole(ctx.Request.Context(), userID, roleID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).
			Str("user_id", userID).
			Str("role_id", roleID).
			Msg("access_control: RemoveRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "role removed successfully"})
}

// BulkAssignRole handles POST /api/access-control/bulk-assign-role.
// @Summary Bulk assign role
// @Description Assigns a role to multiple users.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/access-control/bulk-assign-role [post]
func (c *Controller) BulkAssignRole(ctx *gin.Context) {
	var req dto.BulkAssignRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	if len(req.UserIDs) == 0 || req.RoleID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userIds and roleId are required"))
		return
	}
	if err := c.svc.BulkAssignRole(ctx.Request.Context(), req.UserIDs, req.RoleID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("access_control: BulkAssignRole failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "role assigned successfully"})
}

// GetEffectivePermissions handles GET /api/access-control/users/:userId/effective-permissions.
// @Summary Get effective permissions
// @Description Returns resolved permissions for a user (roles + overrides).
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/effective-permissions [get]
func (c *Controller) GetEffectivePermissions(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	perms, err := c.svc.GetEffectivePermissions(ctx.Request.Context(), userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: GetEffectivePermissions failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, perms)
}

// ListPermissionOverrides handles GET /api/access-control/users/:userId/permission-overrides.
// @Summary List permission overrides
// @Description Returns permission overrides for a specific user.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/permission-overrides [get]
func (c *Controller) ListPermissionOverrides(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	overrides, err := c.svc.ListPermissionOverrides(ctx.Request.Context(), userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: ListPermissionOverrides failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	items := make([]dto.PermissionOverrideResponse, len(overrides))
	for i, override := range overrides {
		items[i] = toOverrideResponse(override)
	}
	response.SuccessGin(ctx, map[string]any{"data": items, "items": items, "total": len(items)})
}

// AddPermissionOverride handles POST /api/access-control/users/:userId/permissions.
// @Summary Add permission override
// @Description Adds a permission override for a user.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/permissions [post]
func (c *Controller) AddPermissionOverride(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	var req dto.AddPermissionOverrideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	override, err := c.svc.AddPermissionOverride(ctx.Request.Context(), userID, req, getUserIDFromCtx(ctx.Request.Context()))
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: AddPermissionOverride failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, toOverrideResponse(override))
}

// DeletePermissionOverride handles DELETE /api/access-control/users/:userId/permissions/:overrideId.
// @Summary Delete permission override
// @Description Removes a permission override.
// @Tags Access Control
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Param overrideId path string true "Override ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/permissions/{overrideId} [delete]
func (c *Controller) DeletePermissionOverride(ctx *gin.Context) {
	overrideID := ctx.Param("overrideId")
	if overrideID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("overrideId is required"))
		return
	}
	if err := c.svc.DeletePermissionOverride(ctx.Request.Context(), overrideID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("override_id", overrideID).Msg("access_control: DeletePermissionOverride failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, map[string]any{"message": "permission override deleted"})
}

// PreviewChanges handles POST /api/access-control/users/:userId/preview.
// @Summary Preview permission changes
// @Description Preview what changes would look like before applying.
// @Tags Access Control
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/access-control/users/{userId}/preview [post]
func (c *Controller) PreviewChanges(ctx *gin.Context) {
	userID := ctx.Param("userId")
	if userID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("userId is required"))
		return
	}
	var req dto.PreviewChangesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid request body"))
		return
	}
	preview, err := c.svc.PreviewChanges(ctx.Request.Context(), userID, req.Changes)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("user_id", userID).Msg("access_control: PreviewChanges failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.SuccessGin(ctx, preview)
}
