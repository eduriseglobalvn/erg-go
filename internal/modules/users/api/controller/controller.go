package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/users/api/request"
	resp "erg.ninja/internal/modules/users/api/response"
	userservice "erg.ninja/internal/modules/users/application/service"
	userrepo "erg.ninja/internal/modules/users/infrastructure/repository"
	platformvalidation "erg.ninja/internal/platform/validation"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

type Controller struct {
	svc    *userservice.Service
	jwtVal *auth.JWTValidator
	log    *logger.Logger
	val    *validator.Validate
}

func New(svc *userservice.Service, jwtVal *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, jwtVal: jwtVal, log: log, val: validator.New()}
}

func (c *Controller) RegisterRoutes(r *gin.Engine) {
	authMw := c.jwtAuthMiddleware()

	me := r.Group("/api/users/me")
	me.Use(authMw)
	me.GET("/", c.handleGetProfile)
	me.PATCH("/", c.handleUpdateProfile)
	me.PUT("/password", c.handleChangePassword)
	me.GET("/sessions", c.handleGetSessions)
	me.DELETE("/sessions/:sessionID", c.handleRevokeSession)
	me.POST("/onboarding", c.handleOnboarding)

	users := r.Group("/api/users")
	users.Use(authMw, middleware.RequireRoles("admin"))
	users.GET("", c.handleListUsers)
	users.GET("/", c.handleListUsers)
	users.GET("/:userID", c.handleGetUserDetail)
	users.PUT("/:userID/status", c.handleUpdateUserStatus)
	users.POST("/:userID/roles", c.handleAssignRoles)
	users.DELETE("/:userID", c.handleDeleteUser)
	users.GET("/:userID/activity", c.handleGetActivity)
	users.POST("/bulk-status", c.handleBulkStatus)
	users.POST("/bulk-delete", c.handleBulkDelete)
}

func (c *Controller) jwtAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.jwtVal == nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		claims, err := c.jwtVal.ValidateRequest(auth.AuthorizationHeaderFromRequest(ctx.Request, ""))
		if err != nil {
			if c.log != nil {
				c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("jwtAuthMiddleware: JWT validation failed")
			}
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		sessionID := auth.SessionIDFromClaims(claims)
		newCtx := context.WithValue(ctx.Request.Context(), ctxKeyUserID, claims.UserID)
		newCtx = context.WithValue(newCtx, middleware.ClaimsKey, claims)

		// Use the tenant already in the context (from TenantMiddleware)
		// falls back to "default" if not set.
		tenantID := "default"
		if tid := tenant.FromContext(ctx.Request.Context()); tid != "" {
			tenantID = tid
		}

		newCtx = context.WithValue(newCtx, ctxKeyTenantID, tenantID)
		newCtx = context.WithValue(newCtx, ctxKeySessionID, sessionID)
		if c.svc != nil {
			if err := c.svc.ValidateActiveSession(newCtx, claims.UserID, sessionID, tenantID); err != nil {
				response.ErrorGin(ctx, http.StatusUnauthorized, "AUTH_SESSION_REPLACED", "session was replaced by a newer login")
				ctx.Abort()
				return
			}
		}
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

// Context keys for middleware-injected values.
type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyTenantID
	ctxKeySessionID
)

func getUserID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok {
		return v
	}
	return ""
}

func getTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTenantID).(string); ok {
		return v
	}
	return ""
}

func getSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeySessionID).(string); ok {
		return v
	}
	return ""
}

// handleGetProfile handles GET /api/users/me.
// @Summary Get current user profile
// @Description Fetch profile details of the currently authenticated user.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} resp.UserResponse
// @Router /api/users/me [get]
func (c *Controller) handleGetProfile(ctx *gin.Context) {
	user, err := c.svc.GetProfile(ctx.Request.Context(), getUserID(ctx.Request.Context()))
	if err != nil {
		response.ErrorGin(ctx, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}
	writeJSON(ctx, http.StatusOK, resp.NewUserResponse(user))
}

// handleUpdateProfile handles PATCH /api/users/me.
// @Summary Update user profile
// @Description Update the profile information of the current user.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body request.UpdateProfileRequest true "Update Data"
// @Success 200 {object} map[string]string
// @Router /api/users/me [patch]
func (c *Controller) handleUpdateProfile(ctx *gin.Context) {
	var req request.UpdateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	if err := c.svc.UpdateProfile(ctx.Request.Context(), getUserID(ctx.Request.Context()), getTenantID(ctx.Request.Context()), &req); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update profile")
		return
	}
	writeJSON(ctx, http.StatusOK, map[string]string{"message": "Profile updated successfully"})
}

// handleChangePassword handles PUT /api/users/me/password.
// @Summary Change password
// @Description Change the current user's password.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /api/users/me/password [put]
func (c *Controller) handleChangePassword(ctx *gin.Context) {
	var req request.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	if err := c.svc.ChangePassword(ctx.Request.Context(), getUserID(ctx.Request.Context()), getTenantID(ctx.Request.Context()), &req); err != nil {
		if errors.Is(err, userrepo.ErrInvalidOldPassword) {
			response.ErrorGin(ctx, http.StatusUnauthorized, "INVALID_PASSWORD", "Current password is incorrect")
			return
		}
		response.ErrorGin(ctx, http.StatusInternalServerError, "PASSWORD_CHANGE_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// handleGetSessions handles GET /api/users/me/sessions.
// @Summary List user sessions
// @Description Returns all active sessions for the current user.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} resp.SessionListResponse
// @Router /api/users/me/sessions [get]
func (c *Controller) handleGetSessions(ctx *gin.Context) {
	sessions, err := c.svc.GetSessions(ctx.Request.Context(), getUserID(ctx.Request.Context()), getTenantID(ctx.Request.Context()))
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "FETCH_SESSIONS_FAILED", err.Error())
		return
	}
	sessionList := make([]resp.SessionResponse, len(sessions))
	for i, s := range sessions {
		sessionList[i] = resp.NewSessionResponse(s, getSessionID(ctx.Request.Context()))
	}
	writeJSON(ctx, http.StatusOK, resp.SessionListResponse{Items: sessionList})
}

// handleRevokeSession handles DELETE /api/users/me/sessions/:sessionID.
// @Summary Revoke a session
// @Description Revokes a specific session.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Session ID"
// @Success 200 {object} resp.RevokeSessionResponse
// @Router /api/users/me/sessions/{sessionID} [delete]
func (c *Controller) handleRevokeSession(ctx *gin.Context) {
	sessionID := ctx.Param("sessionID")
	if err := c.svc.RevokeSession(ctx.Request.Context(), getUserID(ctx.Request.Context()), sessionID, getTenantID(ctx.Request.Context())); err != nil {
		if errors.Is(err, userrepo.ErrSessionNotFound) {
			response.ErrorGin(ctx, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found")
			return
		}
		response.ErrorGin(ctx, http.StatusInternalServerError, "REVOKE_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, resp.RevokeSessionResponse{
		Success:   true,
		RevokedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleOnboarding handles POST /api/users/me/onboarding.
// @Summary Complete onboarding
// @Description Completes the user's onboarding profile steps.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/users/me/onboarding [post]
func (c *Controller) handleOnboarding(ctx *gin.Context) {
	var req request.OnboardingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	if err := c.svc.Onboarding(ctx.Request.Context(), getUserID(ctx.Request.Context()), getTenantID(ctx.Request.Context()), &req); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "ONBOARDING_FAILED", err.Error())
		return
	}
	user, _ := c.svc.GetProfile(ctx.Request.Context(), getUserID(ctx.Request.Context()))
	resp := resp.OnboardingResponse{
		ID:                 user.ID.Hex(),
		Email:              user.Email,
		FullName:           user.FullName,
		AvatarURL:          user.AvatarURL,
		IsProfileCompleted: true,
	}
	writeJSON(ctx, http.StatusOK, resp)
}

// handleListUsers handles GET /api/users.
// @Summary Admin: List users
// @Description Fetch a list of all users in the tenant.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} resp.UserListResponse
// @Router /api/users [get]
func (c *Controller) handleListUsers(ctx *gin.Context) {
	query := &request.ListUsersQuery{
		Page:   parseInt(ctx.Query("page"), 1),
		Limit:  platformvalidation.ClampLimit(parseInt(ctx.Query("limit"), 20), 20, 100),
		Search: ctx.Query("search"),
		Status: ctx.Query("status"),
		Role:   ctx.Query("role"),
	}
	users, total, err := c.svc.ListUsers(ctx.Request.Context(), getTenantID(ctx.Request.Context()), query)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "LIST_USERS_FAILED", err.Error())
		return
	}
	items := make([]resp.UserItemResponse, len(users))
	for i, u := range users {
		items[i] = resp.NewUserItemResponse(&u)
	}
	writeJSON(ctx, http.StatusOK, resp.UserListResponse{
		Users: items,
		Meta: &resp.Meta{
			Page:       query.Page,
			Limit:      query.Limit,
			Total:      total,
			TotalPages: (total + int64(query.Limit) - 1) / int64(query.Limit),
		},
	})
}

// handleGetUserDetail handles GET /api/users/:userID.
// @Summary Admin: Get user detail
// @Description Fetch detailed information of a specific user.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} resp.UserDetailResponse
// @Router /api/users/{userID} [get]
func (c *Controller) handleGetUserDetail(ctx *gin.Context) {
	user, err := c.svc.GetUserDetail(ctx.Request.Context(), ctx.Param("userID"), getTenantID(ctx.Request.Context()))
	if err != nil {
		response.ErrorGin(ctx, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}
	writeJSON(ctx, http.StatusOK, resp.NewUserDetailResponse(user))
}

// handleUpdateUserStatus handles PUT /api/users/:userID/status.
// @Summary Admin: Update user status
// @Description Updates the status (active/banned/suspended) of a user.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} map[string]string
// @Router /api/users/{userID}/status [put]
func (c *Controller) handleUpdateUserStatus(ctx *gin.Context) {
	var body request.AdminUpdateUserStatusRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(body); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	if err := c.svc.UpdateUserStatus(ctx.Request.Context(), ctx.Param("userID"), getTenantID(ctx.Request.Context()), body.Status); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "UPDATE_STATUS_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, map[string]string{"message": "User status updated successfully"})
}

// handleAssignRoles handles POST /api/users/:userID/roles.
// @Summary Admin: Assign roles to user
// @Description Assigns one or more roles to a user.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} map[string]string
// @Router /api/users/{userID}/roles [post]
func (c *Controller) handleAssignRoles(ctx *gin.Context) {
	var req request.AdminAssignRolesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	if err := c.svc.AssignRoles(ctx.Request.Context(), ctx.Param("userID"), getTenantID(ctx.Request.Context()), req.Roles); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "ASSIGN_ROLES_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, map[string]string{"message": "Roles assigned successfully"})
}

// handleDeleteUser handles DELETE /api/users/:userID.
// @Summary Admin: Delete a user
// @Description Permanently deletes a user.
// @Tags Users Admin
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} map[string]string
// @Router /api/users/{userID} [delete]
func (c *Controller) handleDeleteUser(ctx *gin.Context) {
	if err := c.svc.DeleteUser(ctx.Request.Context(), ctx.Param("userID"), getTenantID(ctx.Request.Context())); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "DELETE_USER_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// handleGetActivity handles GET /api/users/:userID/activity.
// @Summary Admin: Get user activity
// @Description Returns activity history for a user (placeholder).
// @Tags Users Admin
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/users/{userID}/activity [get]
func (c *Controller) handleGetActivity(ctx *gin.Context) {
	writeJSON(ctx, http.StatusOK, map[string]any{"activities": []any{}})
}

// handleBulkStatus handles POST /api/users/bulk-status.
// @Summary Admin: Bulk update user status
// @Description Updates status for multiple users at once.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/users/bulk-status [post]
func (c *Controller) handleBulkStatus(ctx *gin.Context) {
	var req request.BulkStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	count, err := c.svc.BulkUpdateStatus(ctx.Request.Context(), req.UserIDs, getTenantID(ctx.Request.Context()), req.Status)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "BULK_STATUS_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, resp.BulkOperationResponse{ModifiedCount: count})
}

// handleBulkDelete handles POST /api/users/bulk-delete.
// @Summary Admin: Bulk delete users
// @Description Deletes multiple users at once.
// @Tags Users Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/users/bulk-delete [post]
func (c *Controller) handleBulkDelete(ctx *gin.Context) {
	var req request.BulkDeleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	if err := c.val.Struct(req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	count, err := c.svc.BulkDelete(ctx.Request.Context(), req.UserIDs, getTenantID(ctx.Request.Context()))
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "BULK_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(ctx, http.StatusOK, resp.BulkOperationResponse{ModifiedCount: count})
}

func writeJSON(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
