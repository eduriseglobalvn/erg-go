// Package tenant provides HTTP middleware for tenant context extraction.
package tenant

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// authClaimsKey is the context key used by erg.ninja/pkg/http/middleware
// to store user JWT claims. It must match the key defined there.
type authClaimsKey struct{}

var userClaimsKey = authClaimsKey{}

// GetUserClaimsFromCtx extracts user claims from context.
// This is a best-effort local copy of the function from
// erg.ninja/pkg/http/middleware to avoid import cycles.
func GetUserClaimsFromCtx(ctx context.Context) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(userClaimsKey)
}

// normalize is a local copy of tenant.Normalize to avoid import cycles.
func normalize(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

// TenantMiddleware extracts a tenant ID from the incoming request using the
// following priority:
//
//  1. X-Tenant-ID header
//  2. JWT claim (tenant_id) — requires auth claims to be already in context
//  3. Subdomain prefix (e.g. "acme" from "acme.erg.ninja")
//
// Returns 400 Bad Request if no tenant ID can be resolved and required is true.
// When required is false, a missing tenant falls back to defaultTenantID.
func TenantMiddleware(required bool, allowSubdomain bool, defaultTenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := resolveTenantID(r, allowSubdomain, defaultTenantID)

			if tenantID == "" && required {
				http.Error(w, `{"error":"tenant_id required","code":"TENANT_ID_REQUIRED"}`, http.StatusBadRequest)
				return
			}

			ctx := WithTenant(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantMiddlewareGin is the Gin-native version of TenantMiddleware.
func TenantMiddlewareGin(required bool, allowSubdomain bool, defaultTenantID string) func(*gin.Context) {
	return func(c *gin.Context) {
		tenantID := resolveTenantID(c.Request, allowSubdomain, defaultTenantID)

		if tenantID == "" && required {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "tenant_id required",
				"code":  "TENANT_ID_REQUIRED",
			})
			return
		}

		ctx := WithTenant(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// resolveTenantID extracts tenant ID from header, JWT, or subdomain.
func resolveTenantID(r *http.Request, allowSubdomain bool, defaultID string) string {
	// Priority 1: explicit header.
	id := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if id != "" {
		return normalize(id)
	}

	// Priority 2: JWT claim — look for tenant_id in the auth context value.
	// erg.ninja/pkg/http/middleware stores claims at authClaimsKey.
	if claims := GetUserClaimsFromCtx(r.Context()); claims != nil {
		if claimsMap, ok := claims.(map[string]interface{}); ok {
			if t, ok := claimsMap["tenant_id"].(string); ok && t != "" {
				return normalize(t)
			}
		}
	}

	// Priority 3: subdomain (optional).
	if allowSubdomain {
		if sub := extractSubdomain(r.Host); sub != "" {
			return normalize(sub)
		}
	}

	// Fallback: configured default.
	return normalize(defaultID)
}

// extractSubdomain returns the leftmost label of a host, stripping the
// standard domain suffix. Returns "" for bare IPs and non-subdomain hosts.
func extractSubdomain(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	parts := strings.Split(host, ".")
	if len(parts) < 2 || len(parts[0]) == 0 {
		return ""
	}
	return parts[0]
}


// TenantFromRequest extracts the tenant ID from a standard http.Request.
func TenantFromRequest(r *http.Request, defaultID string) string {
	return resolveTenantID(r, false, defaultID)
}

// TenantHandler wraps a http.HandlerFunc with tenant context.
func TenantHandler(required bool, defaultID string, next http.HandlerFunc) http.Handler {
	return TenantMiddleware(required, false, defaultID)(next)
}
