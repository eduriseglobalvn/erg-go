// Package middleware provides Gin-compatible HTTP middleware for erg-go.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/cache"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
	Default           int // requests per minute for unknown IPs
}

// rateLimiter implements a simple in-memory sliding window rate limiter.
// For production, use Redis-backed rate limiting for distributed environments.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// newRateLimiter creates a new rate limiter with the given requests per minute limit.
func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Filter out old timestamps.
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	rl.requests[key] = valid

	if len(valid) >= rl.limit {
		return false
	}
	rl.requests[key] = append(valid, now)
	return true
}

// RedisRateLimiter implements rate limiting backed by Redis.
type RedisRateLimiter struct {
	redis *cache.RedisClient
	limit int
}

// NewRedisRateLimiter creates a Redis-backed rate limiter.
func NewRedisRateLimiter(redis *cache.RedisClient, requestsPerMinute int) *RedisRateLimiter {
	return &RedisRateLimiter{
		redis: redis,
		limit: requestsPerMinute,
	}
}

// Allow implements the interface{Allow(string) bool} check.
// Uses Redis INCR + EXPIRE for a simple fixed-window rate limit.
func (r *RedisRateLimiter) Allow(key string) bool {
	// TODO: implement with Redis INCR + EXPIRE
	// redis.Incr(ctx, "ratelimit:"+key)
	// redis.Expire(ctx, "ratelimit:"+key, time.Minute)
	return true
}

// RateLimit creates a Gin middleware for rate limiting.
// If redis is nil, falls back to in-memory rate limiter.
func RateLimit(redis *cache.RedisClient, cfg RateLimitConfig) gin.HandlerFunc {
	var limiter interface{ Allow(string) bool }

	if redis != nil {
		limiter = NewRedisRateLimiter(redis, cfg.RequestsPerMinute)
	} else {
		limiter = newRateLimiter(cfg.RequestsPerMinute)
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "ERR_RATE_LIMITED",
					"message": "Quá nhiều yêu cầu. Vui lòng thử lại sau.",
				},
			})
			return
		}
		c.Next()
	}
}

// RateLimitWithKey creates a rate limiter that uses a custom key function.
func RateLimitWithKey(redis *cache.RedisClient, cfg RateLimitConfig, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFunc(c)
		if key == "" {
			key = c.ClientIP()
		}

		var allowed bool
		if redis != nil {
			rl := NewRedisRateLimiter(redis, cfg.RequestsPerMinute)
			allowed = rl.Allow(key)
		} else {
			rl := newRateLimiter(cfg.RequestsPerMinute)
			allowed = rl.Allow(key)
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "ERR_RATE_LIMITED",
					"message": "Quá nhiều yêu cầu. Vui lòng thử lại sau.",
				},
			})
			return
		}
		c.Next()
	}
}

// IPKeyFunc returns the client IP as the rate limit key.
func IPKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// UserKeyFunc returns the authenticated user ID as the rate limit key.
func UserKeyFunc(c *gin.Context) string {
	return GetUserID(c)
}
