// Package controller handles HTTP requests for the audit module.
package controller

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/audit/api/dto"
	"erg.ninja/internal/modules/audit/application/service"
	"erg.ninja/internal/modules/audit/domain/entity"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for the audit module.
type Controller struct {
	svc          *service.Service
	jwtValidator *auth.JWTValidator
	log          *logger.Logger
}

// NewController creates a new audit controller.
func NewController(svc *service.Service, jwtValidator *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, jwtValidator: jwtValidator, log: log}
}

// RegisterRoutes mounts the audit REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/audit")
	api.Use(c.authMiddleware())
	api.Use(c.requirePermission("system.logs"))

	api.GET("/logs", c.ListLogs)
	api.GET("/logs/:id", c.GetLog)
}

// ─── Auth Middleware ──────────────────────────────────────────────────────────

func (c *Controller) authMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.jwtValidator == nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		claims, err := c.jwtValidator.ValidateRequest(auth.AuthorizationHeaderFromRequest(ctx.Request, ""))
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
				Msg("audit: permission denied")
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

func pageGin(ctx *gin.Context, key string, fallback int) int {
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

func toAuditLogResponse(l *entities.AuditLog) dto.AuditLogResponse {
	if l == nil {
		return dto.AuditLogResponse{}
	}
	return dto.AuditLogResponse{
		ID: l.ID.Hex(), Action: l.Action, ResourceType: l.ResourceType,
		ResourceID: l.ResourceID, UserID: l.UserID,
		UserEmail: l.UserEmail, IPAddress: l.IPAddress,
		UserAgent: l.UserAgent, Changes: l.Changes, Metadata: l.Metadata,
		CreatedAt: l.Timestamp,
	}
}

// ListLogs handles GET /api/audit/logs.
// @Summary List audit logs
// @Description Returns a paginated list of audit logs with filters.
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param action query string false "Filter by action"
// @Param user_id query string false "Filter by user"
// @Param resource_type query string false "Filter by resource"
// @Success 200 {object} response.Response{data=[]dto.AuditLogResponse}
// @Router /api/audit/logs [get]
func (c *Controller) ListLogs(ctx *gin.Context) {
	q := dto.AuditLogQueryParams{
		Page:         pageGin(ctx, "page", 1),
		Limit:        pageGin(ctx, "limit", 20),
		Action:       ctx.Query("action"),
		UserID:       ctx.Query("user_id"),
		ResourceType: ctx.Query("resource_type"),
		StartDate:    ctx.Query("start_date"),
		EndDate:      ctx.Query("end_date"),
	}

	logs, total, err := c.svc.ListLogs(ctx.Request.Context(), q)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("audit: ListLogs failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := make([]dto.AuditLogResponse, len(logs))
	for i, l := range logs {
		items[i] = toAuditLogResponse(l)
	}

	response.PaginatedGin(ctx, items, total, q.Page, q.Limit)
}

// GetLog handles GET /api/audit/logs/:id.
// @Summary Get audit log detail
// @Description Returns the details of a single audit log entry.
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param id path string true "Log ID"
// @Success 200 {object} dto.AuditLogResponse
// @Router /api/audit/logs/{id} [get]
func (c *Controller) GetLog(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}
	log, err := c.svc.GetLog(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("audit: GetLog failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if log == nil {
		response.NotFoundGin(ctx, "audit log not found")
		return
	}
	response.SuccessGin(ctx, toAuditLogResponse(log))
}
