package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestJWTMiddlewareRejectsNilValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTMiddleware(nil))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRequiresCSRFForCookieAuthPost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, token := testValidatorAndToken(t)
	router := gin.New()
	router.Use(JWTMiddleware(validator))
	router.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultAccessTokenCookieName, Value: token})
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestJWTMiddlewareAllowsCookieAuthPostWithCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, token := testValidatorAndToken(t)
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		t.Fatalf("NewCSRFToken() error = %v", err)
	}
	router := gin.New()
	router.Use(JWTMiddleware(validator))
	router.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultAccessTokenCookieName, Value: token})
	req.AddCookie(&http.Cookie{Name: auth.DefaultCSRFTokenCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestJWTMiddlewareAllowsBearerPostWithoutCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, token := testValidatorAndToken(t)
	router := gin.New()
	router.Use(JWTMiddleware(validator))
	router.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func testValidatorAndToken(t *testing.T) (*auth.JWTValidator, string) {
	t.Helper()
	validator, err := auth.NewHS256Validator("0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("NewHS256Validator() error = %v", err)
	}
	token, err := validator.GenerateHS256(&auth.JWTClaims{UserID: "user-1", Subject: "user-1"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateHS256() error = %v", err)
	}
	return validator, token
}
