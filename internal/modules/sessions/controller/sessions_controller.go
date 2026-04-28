// Package controller handles HTTP requests for the sessions module.
package controller

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/sessions/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// contextKey for middleware-injected values.
type contextKey int

const (
	ckUserID contextKey = iota
	ckSessionID
	ckClientIP
)

// Controller handles HTTP requests for sessions.
type Controller struct {
	svc *service.Service
	jwt *auth.JWTValidator
	log *logger.Logger
}

// NewController creates a new sessions controller.
func NewController(svc *service.Service, jwt *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, jwt: jwt, log: log}
}

// RegisterRoutes mounts the sessions REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/sessions")
	api.Use(c.authMiddleware())
	api.GET("/current", c.GetCurrentSession)
}

// authMiddleware validates the JWT token and injects claims into the request context.
func (c *Controller) authMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := c.jwt.Validate(token)
		if err != nil {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}

		sessionID := auth.SessionIDFromClaims(claims)
		if sessionID == "" {
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}

		newCtx := context.WithValue(ctx.Request.Context(), ckUserID, claims.UserID)
		newCtx = context.WithValue(newCtx, ckSessionID, sessionID)
		newCtx = context.WithValue(newCtx, ckClientIP, clientIPGin(ctx))
		ctx.Request = ctx.Request.WithContext(newCtx)
		ctx.Next()
	}
}

// clientIPGin extracts the real client IP from the request (supports reverse proxies).
func clientIPGin(ctx *gin.Context) string {
	if fwd := ctx.GetHeader("X-Forwarded-For"); fwd != "" {
		if idx := strings.IndexByte(fwd, ','); idx >= 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if fwd := ctx.GetHeader("X-Real-IP"); fwd != "" {
		return fwd
	}
	return ctx.ClientIP()
}

// GetCurrentSession handles GET /api/sessions/current.
// @Summary Get current session
// @Description Returns the current user's active session.
// @Tags Sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/sessions/current [get]
func (c *Controller) GetCurrentSession(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())
	userID, _ := ctx.Request.Context().Value(ckUserID).(string)
	sessionID, _ := ctx.Request.Context().Value(ckSessionID).(string)
	clientIP, _ := ctx.Request.Context().Value(ckClientIP).(string)

	if userID == "" || sessionID == "" {
		response.UnauthorizedGin(ctx)
		return
	}

	result, err := c.svc.GetCurrentSession(ctx.Request.Context(), tenantID, userID, sessionID, clientIP)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).
			Str("user_id", userID).
			Str("session_id", sessionID).
			Msg("sessions: GetCurrentSession failed")

		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "pending"):
			response.ErrorGin(ctx, 403, "ACCOUNT_PENDING", "Account is pending approval")
		case strings.Contains(errMsg, "banned"):
			response.ErrorGin(ctx, 403, "ACCOUNT_BANNED", "Account has been banned")
		case strings.Contains(errMsg, "blocked"):
			response.ErrorGin(ctx, 403, "ACCOUNT_BLOCKED", "Account is blocked")
		case service.IsNotFound(err):
			response.NotFoundGin(ctx, "session or user not found")
		default:
			response.InternalErrorGin(ctx, err)
		}
		return
	}

	response.OKGin(ctx, result)
}
