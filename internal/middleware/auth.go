package middleware

import (
	"context"
	"net/http"
	"strings"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

// ctxKey is the type-safe key for values stored in context.
type ctxKey string

const (
	// ClaimsKey is the context key for the authenticated JWT claims.
	ClaimsKey ctxKey = "jwt_claims"
)

// JWTMiddleware returns a Gin middleware that validates Bearer tokens via JWT.
// It extracts the token from the Authorization header, validates it, and injects
// the resulting JWTClaims into the request context under ClaimsKey.
func JWTMiddleware(validator *auth.JWTValidator, skipPaths ...string) gin.HandlerFunc {
	skipSet := buildSkipSet(skipPaths)

	return func(c *gin.Context) {
		// Nil validator = auth disabled (dev/test mode).
		if validator == nil {
			c.Next()
			return
		}

		// Check skip list (supports wildcard prefixes).
		if shouldSkip(c.Request.URL.Path, skipSet) {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing Authorization header",
			})
			return
		}

		claims, err := validator.ValidateRequest(authHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid token",
			})
			return
		}

		// Inject claims into context for downstream handlers.
		ctx := context.WithValue(c.Request.Context(), ClaimsKey, claims)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetClaims extracts JWTClaims from the given context.
// Returns nil if no claims are present (unauthenticated request).
func GetClaims(ctx context.Context) *auth.JWTClaims {
	if claims, ok := ctx.Value(ClaimsKey).(*auth.JWTClaims); ok {
		return claims
	}
	return nil
}

// GetUserID is a convenience helper that returns the user ID from claims,
// or the empty string if no claims are present.
func GetUserID(ctx context.Context) string {
	if claims := GetClaims(ctx); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetRoles returns the roles from claims, or nil.
func GetRoles(ctx context.Context) []string {
	if claims := GetClaims(ctx); claims != nil {
		return claims.Roles
	}
	return nil
}

// writeUnauthorized writes a 401 JSON response.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","message":"` + escapeJSON(message) + `"}`))
}

// buildSkipSet converts a list of path prefixes into a set for O(1) lookup.
func buildSkipSet(paths []string) map[string]bool {
	skipSet := map[string]bool{
		"/healthz": true,
		"/ready":   true,
		"/metrics": true,
		"/swagger": true,
	}
	for _, p := range paths {
		skipSet[p] = true
	}
	return skipSet
}

// shouldSkip returns true if path starts with any prefix in skipSet.
// Supports wildcard suffix (e.g. "/api/admin/*" matches "/api/admin/users").
func shouldSkip(path string, skipSet map[string]bool) bool {
	if skipSet[path] {
		return true
	}
	// Check wildcard suffixes.
	for prefix := range skipSet {
		if strings.HasSuffix(prefix, "*") {
			dir := strings.TrimSuffix(prefix, "*")
			if strings.HasPrefix(path, dir) {
				return true
			}
		}
	}
	return false
}

// escapeJSON escapes a string for safe embedding in a JSON string field.
func escapeJSON(s string) string {
	var b strings.Builder
	for _, ch := range s {
		switch ch {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if ch < ' ' || ch > '~' {
				b.WriteString(" ")
			} else {
				b.WriteRune(ch)
			}
		}
	}
	return b.String()
}
