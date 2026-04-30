package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Role is the type for role identifiers used across the application.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleModerator Role = "moderator"
	RoleEditor    Role = "editor"
	RoleTeacher   Role = "teacher"
	RoleStudent   Role = "student"
	RoleUser      Role = "user"
	RoleGuest     Role = "guest"
)

// RequireRoles returns a middleware that requires at least one of the given roles.
// It reads the authenticated user's roles from the context (set by JWTMiddleware).
func RequireRoles(requiredRoles ...string) gin.HandlerFunc {
	required := make(map[string]bool)
	for _, r := range requiredRoles {
		required[strings.ToLower(r)] = true
	}

	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}

		userRoles := make(map[string]bool)
		for _, role := range claims.Roles {
			userRoles[strings.ToLower(role)] = true
		}

		// Admin bypasses all role checks.
		if userRoles["admin"] {
			c.Next()
			return
		}

		// Check for at least one required role.
		for _, req := range requiredRoles {
			if userRoles[strings.ToLower(req)] {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "insufficient permissions",
		})
	}
}

// RequirePermission returns a middleware that checks for a specific permission string
// within the JWT claims' Permissions field.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}

		for _, p := range claims.Permissions {
			if p == permission || p == "*" {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "permission '" + permission + "' required",
		})
	}
}

// RequireUserID returns a middleware that ensures a request is authenticated
// and optionally verifies the user ID matches a path parameter.
func RequireUserID(pathParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}

		if pathParam != "" {
			id := c.Param(pathParam)
			if id != "" && id != claims.UserID {
				// Admins can access any user's resource.
				if !hasRole(claims.Roles, RoleAdmin) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error":   "forbidden",
						"message": "access denied",
					})
					return
				}
			}
		}

		c.Next()
	}
}

// hasRole reports whether the roles slice contains the given role.
func hasRole(roles []string, role Role) bool {
	for _, r := range roles {
		if strings.EqualFold(r, string(role)) {
			return true
		}
	}
	return false
}

// RequireScope returns a middleware that checks the JWT 'scope' or 'scopes' claim.
func RequireScope(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		for _, p := range claims.Permissions {
			if p == requiredScope || strings.HasPrefix(p, requiredScope+":") {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "scope '" + requiredScope + "' required",
		})
	}
}

// FilterByUserID is a helper for repository/service layer to scope queries to
// the authenticated user, unless the caller is an admin.
func FilterByUserID(ctx context.Context) (userID string, isAdmin bool) {
	claims := GetClaims(ctx)
	if claims == nil {
		return "", false
	}
	for _, role := range claims.Roles {
		if strings.EqualFold(string(role), string(RoleAdmin)) {
			return claims.UserID, true
		}
	}
	return claims.UserID, false
}
