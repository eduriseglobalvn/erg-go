package middleware

import (
	"net/http"

	"erg.ninja/internal/modules/access_control/domain/policy"
	"github.com/gin-gonic/gin"
)

// RequirePortal requires the authenticated subject to be allowed into one of
// the enterprise portals: lms, elearning, hoclieu, or cms.
func RequirePortal(portal string) gin.HandlerFunc {
	requiredPortal := policy.NormalizePortal(policy.Portal(portal))
	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		if !policy.ValidPortal(requiredPortal) || !policy.SubjectHasPortal(policy.SubjectFromClaims(claims), requiredPortal) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "portal '" + string(requiredPortal) + "' required",
			})
			return
		}
		c.Next()
	}
}

// RequireAccessPermission applies the enterprise policy decision helper for a
// concrete permission. It honors role-derived grants, wildcard grants, aliases,
// and explicit deny overrides; chain RequirePortal when portal scope is needed.
func RequireAccessPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c.Request.Context())
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}

		decision := policy.Decide(policy.Request{
			Subject:  policy.SubjectFromClaims(claims),
			Resource: policy.Resource{Permission: permission},
		})
		if decision.Allowed {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "permission '" + permission + "' required",
		})
	}
}
