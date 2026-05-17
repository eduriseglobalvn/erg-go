package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

const (
	DefaultCSRFTokenCookieName = "erg_csrf_token"
	CSRFHeaderName             = "X-CSRF-Token"
)

var ErrCSRFTokenInvalid = errors.New("auth: csrf token missing or invalid")

func NewCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func UnsafeMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func UsesCookieAuth(r *http.Request, accessCookieName string) bool {
	if r == nil || strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return false
	}
	return AccessTokenFromRequest(r, accessCookieName) != ""
}

func UsesRefreshCookie(r *http.Request, refreshCookieName string) bool {
	if r == nil {
		return false
	}
	return RefreshTokenFromRequest(r, refreshCookieName) != ""
}

func ValidateCSRF(r *http.Request) error {
	if r == nil || !UnsafeMethod(r.Method) {
		return nil
	}
	header := strings.TrimSpace(r.Header.Get(CSRFHeaderName))
	if header == "" {
		return ErrCSRFTokenInvalid
	}
	cookie, err := r.Cookie(DefaultCSRFTokenCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return ErrCSRFTokenInvalid
	}
	if !constantTimeStringEqual(header, cookie.Value) {
		return ErrCSRFTokenInvalid
	}
	return nil
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
