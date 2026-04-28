// Package tenant provides multi-tenant context propagation utilities.
package tenant

import (
	"context"
)

// ctxKey is the unexported context value key for tenant ID.
type ctxKey struct{}

// contextKey is the single sentinel used to store tenant ID in context.
// Using an unexported struct type prevents collisions.
var contextKey = &ctxKey{}

// WithTenant returns a new context that carries the given tenant ID.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey, tenantID)
}

// FromContext extracts the tenant ID from ctx.
// Returns "" if ctx is nil or no tenant ID is stored.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(contextKey)
	if v == nil {
		return ""
	}
	id, ok := v.(string)
	if !ok {
		return ""
	}
	return id
}

// MustFromContext extracts the tenant ID or panics if not set.
// Use in request handlers after TenantMiddleware has been applied.
func MustFromContext(ctx context.Context) string {
	id := FromContext(ctx)
	if id == "" {
		panic("tenant: missing in context — ensure TenantMiddleware is applied")
	}
	return id
}

// IsValid checks whether a tenant ID is non-empty and safe for use as
// a collection/queue/Redis-key prefix. Returns false for IDs containing
// colons, underscores at the start, or reserved keywords.
func IsValid(tenantID string) bool {
	if tenantID == "" {
		return false
	}
	// Reject reserved prefixes and characters used in key schemes.
	if len(tenantID) > 64 {
		return false
	}
	for i, c := range tenantID {
		if c == ':' || c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			return false
		}
		// Disallow leading underscore (reserved for internal use).
		if i == 0 && c == '_' {
			return false
		}
	}
	return true
}

// Normalize lowercases and trims whitespace from a tenant ID.
func Normalize(tenantID string) string {
	return trimAndLower(tenantID)
}

func trimAndLower(s string) string {
	// Manual trim to avoid importing strings for this one helper.
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
