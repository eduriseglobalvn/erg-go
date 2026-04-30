package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitWithKeyKeepsStateAcrossRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitWithKey(nil, RateLimitConfig{RequestsPerMinute: 1}, func(*gin.Context) string {
		return "same-user"
	}))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", second.Code)
	}
}

func TestRateLimitDefaultsInvalidLimit(t *testing.T) {
	limiter := newRateLimiter(0)
	for i := 0; i < 60; i++ {
		if !limiter.Allow("client") {
			t.Fatalf("request %d unexpectedly blocked", i+1)
		}
	}
	if limiter.Allow("client") {
		t.Fatal("expected default 60 rpm limiter to block request 61")
	}
}
