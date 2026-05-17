package auth

import (
	"net/http"
	"strings"
)

const (
	DefaultAccessTokenCookieName  = "erg_access_token"
	DefaultRefreshTokenCookieName = "erg_refresh_token"
	LegacyAccessTokenCookieName   = "accessToken"
	LegacyRefreshTokenCookieName  = "refreshToken"
)

// AuthorizationHeaderFromRequest returns the Bearer authorization header from
// either the explicit Authorization header or the HttpOnly access-token cookie.
func AuthorizationHeaderFromRequest(r *http.Request, cookieName string) string {
	if r == nil {
		return ""
	}
	if header := strings.TrimSpace(r.Header.Get("Authorization")); header != "" {
		return header
	}
	token := AccessTokenFromRequest(r, cookieName)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

// AccessTokenFromRequest reads the access token from the configured cookie,
// with a legacy fallback for the previous Next proxy cookie name.
func AccessTokenFromRequest(r *http.Request, cookieName string) string {
	if r == nil {
		return ""
	}
	for _, name := range uniqueCookieNames(cookieName, DefaultAccessTokenCookieName, LegacyAccessTokenCookieName) {
		if cookie, err := r.Cookie(name); err == nil {
			if value := strings.TrimSpace(cookie.Value); value != "" {
				return value
			}
		}
	}
	return ""
}

// RefreshTokenFromRequest reads the refresh token from the configured cookie,
// with a legacy fallback for the previous Next proxy cookie name.
func RefreshTokenFromRequest(r *http.Request, cookieName string) string {
	if r == nil {
		return ""
	}
	for _, name := range uniqueCookieNames(cookieName, DefaultRefreshTokenCookieName, LegacyRefreshTokenCookieName) {
		if cookie, err := r.Cookie(name); err == nil {
			if value := strings.TrimSpace(cookie.Value); value != "" {
				return value
			}
		}
	}
	return ""
}

func uniqueCookieNames(names ...string) []string {
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
