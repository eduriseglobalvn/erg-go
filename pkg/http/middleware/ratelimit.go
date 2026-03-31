package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiter settings.
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
}

// limiterOption is a functional option for RateLimitMiddleware.
type limiterOption func(*rateLimiter)

type rateLimiter struct {
	requests    int
	lastReset   time.Time
	mu          sync.Mutex
	requestsMin int
	burst       int
}

// RateLimitMiddleware returns an in-memory rate limiter middleware.
// Default: 100 requests per minute per IP, burst of 20.
func RateLimitMiddleware(opts ...limiterOption) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		requestsMin: 100,
		burst:       20,
		lastReset:   time.Now(),
	}
	for _, o := range opts {
		o(rl)
	}
	if rl.requestsMin <= 0 {
		rl.requestsMin = 100
	}
	if rl.burst <= 0 {
		rl.burst = 20
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetRealIP(r.Context())
			if ip == "" {
				ip = r.RemoteAddr
			}

			allowed, remaining, resetAt := rl.allow(ip)
			w.Header().Set("X-RateLimit-Limit", stringOfInt(rl.requestsMin))
			w.Header().Set("X-RateLimit-Remaining", stringOfInt(remaining))
			w.Header().Set("X-RateLimit-Reset", stringOfInt(int(resetAt.Unix())))

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded","retry_after":` +
					stringOfInt(int(time.Until(resetAt).Seconds())) + `}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WithRateLimitRequests sets the requests per minute for the rate limiter.
func WithRateLimitRequests(n int) limiterOption {
	return func(rl *rateLimiter) {
		rl.requestsMin = n
	}
}

// WithRateLimitBurst sets the burst size for the rate limiter.
func WithRateLimitBurst(n int) limiterOption {
	return func(rl *rateLimiter) {
		rl.burst = n
	}
}

// allow checks if a request from the given IP is allowed.
// Returns: allowed, remaining requests, time when the rate limit resets.
func (rl *rateLimiter) allow(ip string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	window := now.Truncate(time.Minute)

	// Reset if we're in a new window.
	if window.After(rl.lastReset) {
		rl.requests = 0
		rl.lastReset = window
	}

	resetAt := window.Add(time.Minute)

	// Check burst capacity first (allow burst even if rate limit is hit).
	if rl.requests < rl.requestsMin+rl.burst {
		rl.requests++
		remaining := rl.requestsMin + rl.burst - rl.requests
		if remaining < 0 {
			remaining = 0
		}
		allowed := rl.requests <= rl.requestsMin
		return allowed, remaining, resetAt
	}

	return false, 0, resetAt
}

// stringOfInt converts an int to a string without importing strconv.
func stringOfInt(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// GlobalInMemoryLimiter is a package-level limiter map for shared rate limiting state.
type GlobalInMemoryLimiter struct {
	limiters map[string]*rateLimiter
	mu       sync.Mutex
	opts     limiterOption
}

var globalLimiters = &GlobalInMemoryLimiter{
	limiters: make(map[string]*rateLimiter),
	opts:     WithRateLimitRequests(100),
}

// Acquire checks if a request from the given IP can proceed globally.
func (g *GlobalInMemoryLimiter) Acquire(ctx context.Context, ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	lim, ok := g.limiters[ip]
	if !ok {
		lim = &rateLimiter{requestsMin: 100, burst: 20, lastReset: time.Now()}
		g.limiters[ip] = lim
	}

	allowed, _, _ := lim.allow(ip)
	return allowed
}
