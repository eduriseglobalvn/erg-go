package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	authr "erg.ninja/internal/modules/auth/api/response"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"
)

const loggedInCookieName = "isLoggedIn"

func ctrlAccessCookieName(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.Auth.Cookie.AccessTokenName) != "" {
		return strings.TrimSpace(cfg.Auth.Cookie.AccessTokenName)
	}
	return auth.DefaultAccessTokenCookieName
}

func ctrlRefreshCookieName(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.Auth.Cookie.RefreshTokenName) != "" {
		return strings.TrimSpace(cfg.Auth.Cookie.RefreshTokenName)
	}
	return auth.DefaultRefreshTokenCookieName
}

func (c *Controller) setAuthCookies(ctx *gin.Context, accessToken, refreshToken string, user authr.ProfileResponse) {
	_ = user
	c.setTokenCookies(ctx, accessToken, refreshToken)
}

func (c *Controller) setTokenCookies(ctx *gin.Context, accessToken, refreshToken string) {
	if !c.cookieEnabled() {
		return
	}
	if accessToken != "" || refreshToken != "" {
		if token, err := auth.NewCSRFToken(); err == nil {
			c.setReadableCookie(ctx, auth.DefaultCSRFTokenCookieName, token, c.refreshCookieMaxAge())
		}
	}
	if accessToken != "" {
		c.setCookie(ctx, ctrlAccessCookieName(c.cfg), accessToken, c.accessCookieMaxAge(), true)
		c.setReadableCookie(ctx, loggedInCookieName, "true", c.refreshCookieMaxAge())
	}
	if refreshToken != "" {
		c.setCookie(ctx, ctrlRefreshCookieName(c.cfg), refreshToken, c.refreshCookieMaxAge(), true)
		c.setReadableCookie(ctx, loggedInCookieName, "true", c.refreshCookieMaxAge())
	}
}

func (c *Controller) clearAuthCookies(ctx *gin.Context) {
	for _, name := range []string{
		ctrlAccessCookieName(c.cfg),
		ctrlRefreshCookieName(c.cfg),
		auth.LegacyAccessTokenCookieName,
		auth.LegacyRefreshTokenCookieName,
		auth.DefaultCSRFTokenCookieName,
		loggedInCookieName,
		"clientUserId",
		"authProvider",
		"accountType",
	} {
		c.clearCookie(ctx, name, true)
		c.clearCookie(ctx, name, false)
	}
}

func (c *Controller) setReadableCookie(ctx *gin.Context, name, value string, maxAge int) {
	c.setCookie(ctx, name, value, maxAge, false)
}

func (c *Controller) setCookie(ctx *gin.Context, name, value string, maxAge int, httpOnly bool) {
	if ctx == nil || strings.TrimSpace(name) == "" {
		return
	}
	// #nosec G124 -- Secure is config-driven for local HTTP development; production config validation requires secure cookies.
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   c.cookieDomain(),
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: httpOnly,
		Secure:   c.cookieSecure(),
		SameSite: c.cookieSameSite(),
	}
	http.SetCookie(ctx.Writer, cookie)
}

func (c *Controller) clearCookie(ctx *gin.Context, name string, httpOnly bool) {
	if ctx == nil || strings.TrimSpace(name) == "" {
		return
	}
	// #nosec G124 -- clearing must mirror both HttpOnly and readable cookies; production config validation requires Secure.
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   c.cookieDomain(),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: httpOnly,
		Secure:   c.cookieSecure(),
		SameSite: c.cookieSameSite(),
	})
}

func (c *Controller) cookieEnabled() bool {
	return c == nil || c.cfg == nil || c.cfg.Auth.Cookie.Enabled
}

func (c *Controller) cookieDomain() string {
	if c == nil || c.cfg == nil {
		return ""
	}
	return strings.TrimSpace(c.cfg.Auth.Cookie.Domain)
}

func (c *Controller) cookieSecure() bool {
	return c != nil && c.cfg != nil && c.cfg.Auth.Cookie.Secure
}

func (c *Controller) cookieSameSite() http.SameSite {
	if c == nil || c.cfg == nil {
		return http.SameSiteLaxMode
	}
	switch strings.ToLower(strings.TrimSpace(c.cfg.Auth.Cookie.SameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func (c *Controller) accessCookieMaxAge() int {
	if c != nil && c.cfg != nil && c.cfg.Auth.AccessTokenTTL > 0 {
		return int(c.cfg.Auth.AccessTokenTTL.Seconds())
	}
	return int((15 * time.Minute).Seconds())
}

func (c *Controller) refreshCookieMaxAge() int {
	if c != nil && c.cfg != nil && c.cfg.Auth.RefreshTokenTTL > 0 {
		return int(c.cfg.Auth.RefreshTokenTTL.Seconds())
	}
	return int((7 * 24 * time.Hour).Seconds())
}

func (c *Controller) shouldSuppressTokens(ctx *gin.Context) bool {
	return false
}

func sanitizedAuthResponse(resp *authr.AuthResponse) *authr.AuthResponse {
	if resp == nil {
		return nil
	}
	copy := *resp
	copy.AccessToken = ""
	copy.RefreshToken = ""
	return &copy
}

func sanitizedTokenResponse(resp *authr.TokenResponse) *authr.TokenResponse {
	if resp == nil {
		return nil
	}
	copy := *resp
	copy.AccessToken = ""
	copy.RefreshToken = ""
	return &copy
}
