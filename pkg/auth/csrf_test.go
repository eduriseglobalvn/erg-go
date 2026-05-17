package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateCSRFRequiresHeaderAndCookieForUnsafeMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := ValidateCSRF(req); err != ErrCSRFTokenInvalid {
		t.Fatalf("expected ErrCSRFTokenInvalid, got %v", err)
	}
}

func TestValidateCSRFPassesMatchingDoubleSubmitToken(t *testing.T) {
	token, err := NewCSRFToken()
	if err != nil {
		t.Fatalf("NewCSRFToken() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(CSRFHeaderName, token)
	req.AddCookie(&http.Cookie{Name: DefaultCSRFTokenCookieName, Value: token})
	if err := ValidateCSRF(req); err != nil {
		t.Fatalf("ValidateCSRF() error = %v", err)
	}
}

func TestValidateCSRFSkipsSafeMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := ValidateCSRF(req); err != nil {
		t.Fatalf("ValidateCSRF() error = %v", err)
	}
}
