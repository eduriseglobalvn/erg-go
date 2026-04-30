package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRealIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RealIPWithTrustedProxies([]string{"10.0.0.0/8"}))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRealIP(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "203.0.113.10" {
		t.Fatalf("expected untrusted peer IP, got %q", got)
	}
}

func TestRealIPUsesForwardedForFromTrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RealIPWithTrustedProxies([]string{"10.0.0.0/8"}))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRealIP(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:4567"
	req.Header.Set("X-Forwarded-For", "198.51.100.99, 10.1.2.3")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "198.51.100.99" {
		t.Fatalf("expected forwarded client IP, got %q", got)
	}
}

func TestRealIPUsesXRealIPFromTrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RealIPWithTrustedProxies([]string{"127.0.0.1"}))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRealIP(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-Real-IP", "198.51.100.77")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "198.51.100.77" {
		t.Fatalf("expected X-Real-IP client IP, got %q", got)
	}
}
