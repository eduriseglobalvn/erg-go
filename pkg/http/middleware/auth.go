package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"erg.ninja/pkg/auth"
)

// UserClaims holds the JWT claims extracted by the auth middleware.
type UserClaims struct {
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Email       string   `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// ClaimsKey is the context key for user claims.
type ClaimsKey struct{}

var UserClaimsKey = ClaimsKey{}

// AuthMiddleware validates JWT tokens from the Authorization header.
// It extracts claims and stores them in the request context.
func AuthMiddleware(validator *auth.JWTValidator, skipPaths ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this path should skip auth.
			path := r.URL.Path
			for _, skip := range skipPaths {
				if path == skip || strings.HasPrefix(path, skip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Bearer token.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := validator.Validate(tokenString)
			if err != nil {
				http.Error(w, `{"error":"invalid token","details":"`+err.Error()+`"}`, http.StatusUnauthorized)
				return
			}

			// Store claims in context.
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserClaims extracts user claims from the context.
func GetUserClaims(ctx context.Context) *UserClaims {
	if ctx == nil {
		return nil
	}
	if claims, ok := ctx.Value(UserClaimsKey).(*UserClaims); ok {
		return claims
	}
	return nil
}

// AuthMiddlewareFromConfig creates auth middleware using config values.
func AuthMiddlewareFromConfig(cfg struct {
	SkipPaths []string
}, validator *auth.JWTValidator) func(http.Handler) http.Handler {
	return AuthMiddleware(validator, cfg.SkipPaths...)
}

// RequirePermission returns a middleware that checks for a specific permission.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			for _, p := range claims.Permissions {
				if p == permission || p == "*" {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden","missing_permission":"`+permission+`"}`, http.StatusForbidden)
		})
	}
}

// RequireRole returns a middleware that checks for a specific role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			for _, userRole := range claims.Roles {
				if userRole == role || userRole == "*" {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden","missing_role":"`+role+`"}`, http.StatusForbidden)
		})
	}
}

// chiRouterAuth is a helper to adapt chi router to the auth middleware signature.
func chiRouterAuth(validator *auth.JWTValidator, skipPaths []string) func(next http.Handler) http.Handler {
	return AuthMiddleware(validator, skipPaths...)
}

// UserIDFromContext extracts the user ID from the JWT claims in the context.
func UserIDFromContext(ctx context.Context) string {
	claims := GetUserClaims(ctx)
	if claims == nil {
		return ""
	}
	return claims.UserID
}
