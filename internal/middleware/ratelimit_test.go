package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
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

func TestAuthRateLimitRedisErrorFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rateLimitWithChecker(stubRateLimitChecker{
		decision: rateLimitDecision{Err: errors.New("redis down"), Backend: "redis"},
	}, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@erg.edu.vn","deviceId":"dev-1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.Code)
	}
}

func TestPublicReadRateLimitRedisErrorFailsOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rateLimitWithChecker(stubRateLimitChecker{
		decision: rateLimitDecision{Err: errors.New("redis down"), Backend: "redis"},
	}, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.GET("/api/posts", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
}

func TestAuthRateLimitRestoresRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"email":"admin@erg.edu.vn","deviceId":"dev-1"}`
	r := gin.New()
	r.Use(rateLimitWithChecker(stubRateLimitChecker{
		decision: rateLimitDecision{Allowed: true, Limit: 10, Remaining: 9, Backend: "redis"},
	}, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.POST("/api/auth/login", func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(raw) != body {
			t.Fatalf("body = %q, want %q", string(raw), body)
		}
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
}

func TestAuthLoginUsesMultipleRateLimitDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingRateLimitChecker{}
	r := gin.New()
	r.Use(rateLimitWithChecker(checker, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@erg.edu.vn","deviceId":"dev-1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	want := []string{"auth_ip", "auth_identity", "auth_device"}
	if len(checker.policies) != len(want) {
		t.Fatalf("policies = %v, want %v", checker.policies, want)
	}
	for i := range want {
		if checker.policies[i] != want[i] {
			t.Fatalf("policy[%d] = %q, want %q", i, checker.policies[i], want[i])
		}
	}
}

func TestAssetLaunchUsesRoutePolicyAndUserAssetKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingRateLimitChecker{}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), ClaimsKey, &auth.JWTClaims{UserID: "teacher-1"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(rateLimitWithChecker(checker, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.GET("/api/hoclieu/assets/:assetId/launch", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/assets/asset-abc/launch", nil)
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if len(checker.policies) != 1 || checker.policies[0] != "hoclieu_asset_launch" {
		t.Fatalf("policies = %v, want [hoclieu_asset_launch]", checker.policies)
	}
	if len(checker.keys) != 1 || checker.keys[0] != "user:teacher-1:asset:asset-abc:launch" {
		t.Fatalf("keys = %v, want user:teacher-1:asset:asset-abc:launch", checker.keys)
	}
}

func TestQuizSubmitUsesRoutePolicyAndAttemptKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingRateLimitChecker{}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), ClaimsKey, &auth.JWTClaims{UserID: "student-1"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(rateLimitWithChecker(checker, RateLimitConfig{RequestsPerMinute: 100}, nil))
	r.POST("/api/lms/attempts/:attemptId/submit", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/lms/attempts/attempt-123/submit", nil)
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if len(checker.policies) != 1 || checker.policies[0] != "lms_quiz_submit" {
		t.Fatalf("policies = %v, want [lms_quiz_submit]", checker.policies)
	}
	if len(checker.keys) != 1 || checker.keys[0] != "user:student-1:attempt:attempt-123:submit" {
		t.Fatalf("keys = %v, want user:student-1:attempt:attempt-123:submit", checker.keys)
	}
}

type stubRateLimitChecker struct {
	decision rateLimitDecision
}

func (s stubRateLimitChecker) Allow(context.Context, string, rateLimitPolicy) rateLimitDecision {
	return s.decision
}

type recordingRateLimitChecker struct {
	policies []string
	keys     []string
}

func (r *recordingRateLimitChecker) Allow(_ context.Context, key string, policy rateLimitPolicy) rateLimitDecision {
	r.policies = append(r.policies, policy.Name)
	r.keys = append(r.keys, key)
	return rateLimitDecision{Allowed: true, Limit: effectiveRateLimit(policy), Remaining: effectiveRateLimit(policy) - 1, Backend: "redis"}
}
