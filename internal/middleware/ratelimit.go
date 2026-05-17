// Package middleware provides Gin-compatible HTTP middleware for erg-go.
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"

	"erg.ninja/pkg/cache"
)

const (
	defaultRateLimitWindow = time.Minute
	authBodyReadLimit      = 64 << 10
)

var rateLimitMemberSeq uint64

var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call("ZREMRANGEBYSCORE", key, 0, now - window)
local current = tonumber(redis.call("ZCARD", key))
if current < limit then
	redis.call("ZADD", key, now, member)
	redis.call("PEXPIRE", key, window)
	return {1, limit - current - 1, 0, window}
end

local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
local retry_after = window
if oldest[2] ~= nil then
	retry_after = window - (now - tonumber(oldest[2]))
	if retry_after < 0 then
		retry_after = 0
	end
end
return {0, 0, retry_after, window}
`)

var (
	rateLimitRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "erg",
		Subsystem: "http",
		Name:      "rate_limit_requests_total",
		Help:      "Rate limit decisions by policy, route, backend and outcome.",
	}, []string{"policy", "route", "backend", "outcome"})
	rateLimitDecisionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "erg",
		Subsystem: "http",
		Name:      "rate_limit_decision_duration_seconds",
		Help:      "Rate limit decision latency by policy, route and backend.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"policy", "route", "backend"})
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
	Default           int // requests per minute for unknown IPs
	SkipLoopback      bool
}

type rateLimitPolicy struct {
	Name     string
	Limit    int
	Burst    int
	Window   time.Duration
	FailOpen bool
}

type rateLimitCheck struct {
	Policy rateLimitPolicy
	Key    string
}

type rateLimitDecision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
	ResetAfter time.Duration
	Err        error
	Backend    string
}

type rateLimitChecker interface {
	Allow(context.Context, string, rateLimitPolicy) rateLimitDecision
}

// rateLimiter implements an in-memory sliding-window limiter for local/dev
// fallback. It is per-process and must not be the production distributed gate.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// newRateLimiter creates a local sliding-window limiter.
func newRateLimiter(requestsPerMinute int) *rateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    requestsPerMinute,
		window:   defaultRateLimitWindow,
	}
}

// Allow checks if a request from the given key is allowed using the default policy.
func (rl *rateLimiter) Allow(key string) bool {
	policy := rateLimitPolicy{Name: "default", Limit: rl.limit, Window: defaultRateLimitWindow, FailOpen: true}
	return rl.allow(context.Background(), key, policy).Allowed
}

func (rl *rateLimiter) allow(_ context.Context, key string, policy rateLimitPolicy) rateLimitDecision {
	policy = normalizeRateLimitPolicy(policy)
	compoundKey := policy.Name + ":" + key

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-policy.Window)
	valid := rl.requests[compoundKey][:0]
	for _, t := range rl.requests[compoundKey] {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	limit := effectiveRateLimit(policy)
	if len(valid) >= limit {
		retryAfter := policy.Window
		if len(valid) > 0 {
			retryAfter = policy.Window - now.Sub(valid[0])
			if retryAfter < 0 {
				retryAfter = 0
			}
		}
		rl.requests[compoundKey] = valid
		return rateLimitDecision{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			RetryAfter: retryAfter,
			ResetAfter: retryAfter,
			Backend:    "memory",
		}
	}

	valid = append(valid, now)
	rl.requests[compoundKey] = valid
	return rateLimitDecision{
		Allowed:    true,
		Limit:      limit,
		Remaining:  limit - len(valid),
		RetryAfter: 0,
		ResetAfter: policy.Window,
		Backend:    "memory",
	}
}

// RedisRateLimiter implements a distributed Redis sliding-window limiter.
type RedisRateLimiter struct {
	redis *cache.RedisClient
	limit int
}

// NewRedisRateLimiter creates a Redis-backed rate limiter.
func NewRedisRateLimiter(redisClient *cache.RedisClient, requestsPerMinute int) *RedisRateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	return &RedisRateLimiter{
		redis: redisClient,
		limit: requestsPerMinute,
	}
}

// Allow implements the legacy interface{Allow(string) bool} check.
func (r *RedisRateLimiter) Allow(key string) bool {
	policy := rateLimitPolicy{Name: "default", Limit: r.limit, Window: defaultRateLimitWindow, FailOpen: false}
	return r.allow(context.Background(), key, policy).Allowed
}

func (r *RedisRateLimiter) allow(ctx context.Context, key string, policy rateLimitPolicy) rateLimitDecision {
	policy = normalizeRateLimitPolicy(policy)
	if r == nil || r.redis == nil || r.redis.Client() == nil {
		return rateLimitDecision{Allowed: policy.FailOpen, Limit: effectiveRateLimit(policy), Err: errRateLimiterUnavailable(), Backend: "redis"}
	}

	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	now := time.Now().UnixMilli()
	limit := effectiveRateLimit(policy)
	member := strconv.FormatInt(now, 10) + ":" + strconv.FormatUint(atomic.AddUint64(&rateLimitMemberSeq, 1), 10)
	keyName := "ratelimit:v2:" + policy.Name + ":" + hashRateLimitPart(key)

	values, err := rateLimitScript.Run(ctx, r.redis.Client(), []string{keyName},
		now,
		policy.Window.Milliseconds(),
		limit,
		member,
	).Slice()
	if err != nil {
		return rateLimitDecision{Allowed: policy.FailOpen, Limit: limit, Err: err, Backend: "redis"}
	}

	allowed := int64Result(values, 0) == 1
	retryAfter := time.Duration(int64Result(values, 2)) * time.Millisecond
	resetAfter := time.Duration(int64Result(values, 3)) * time.Millisecond
	return rateLimitDecision{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  int(int64Result(values, 1)),
		RetryAfter: retryAfter,
		ResetAfter: resetAfter,
		Backend:    "redis",
	}
}

type redisBackedChecker struct {
	limiter *RedisRateLimiter
}

func (c redisBackedChecker) Allow(ctx context.Context, key string, policy rateLimitPolicy) rateLimitDecision {
	return c.limiter.allow(ctx, key, policy)
}

type memoryBackedChecker struct {
	limiter *rateLimiter
}

func (c memoryBackedChecker) Allow(ctx context.Context, key string, policy rateLimitPolicy) rateLimitDecision {
	return c.limiter.allow(ctx, key, policy)
}

// RateLimit creates a Gin middleware for distributed rate limiting.
func RateLimit(redisClient *cache.RedisClient, cfg RateLimitConfig) gin.HandlerFunc {
	var checker rateLimitChecker
	if redisClient != nil {
		checker = redisBackedChecker{limiter: NewRedisRateLimiter(redisClient, cfg.RequestsPerMinute)}
	} else {
		checker = memoryBackedChecker{limiter: newRateLimiter(cfg.RequestsPerMinute)}
	}
	return rateLimitWithChecker(checker, cfg, nil)
}

// RateLimitWithKey creates a rate limiter that uses a custom key function.
func RateLimitWithKey(redisClient *cache.RedisClient, cfg RateLimitConfig, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	var checker rateLimitChecker
	if redisClient != nil {
		checker = redisBackedChecker{limiter: NewRedisRateLimiter(redisClient, cfg.RequestsPerMinute)}
	} else {
		checker = memoryBackedChecker{limiter: newRateLimiter(cfg.RequestsPerMinute)}
	}
	return rateLimitWithChecker(checker, cfg, keyFunc)
}

func rateLimitWithChecker(checker rateLimitChecker, cfg RateLimitConfig, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipRateLimit(c) {
			c.Next()
			return
		}

		if cfg.SkipLoopback && isLoopbackIP(clientIPKey(c)) {
			c.Next()
			return
		}

		checks := rateLimitChecksForRequest(c, cfg, keyFunc)
		for _, check := range checks {
			start := time.Now()
			decision := checker.Allow(c.Request.Context(), check.Key, check.Policy)
			observeRateLimitDecision(c, check.Policy, decision, time.Since(start))

			if decision.Err != nil && check.Policy.FailOpen {
				continue
			}
			if decision.Err != nil {
				writeRateLimitUnavailable(c, decision)
				return
			}
			if !decision.Allowed {
				writeRateLimited(c, decision)
				return
			}
			addRateLimitHeaders(c, decision)
		}
		c.Next()
	}
}

func rateLimitChecksForRequest(c *gin.Context, cfg RateLimitConfig, keyFunc func(*gin.Context) string) []rateLimitCheck {
	if keyFunc != nil {
		key := keyFunc(c)
		if key == "" {
			key = clientIPKey(c)
		}
		return []rateLimitCheck{{Policy: basePolicy(cfg, routePolicyName(c), routeFailOpen(c)), Key: key}}
	}

	path := ""
	method := ""
	if c != nil && c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
		method = c.Request.Method
	}

	ip := clientIPKey(c)
	if isAuthLoginPath(path) && method == http.MethodPost {
		identity, device := authIdentityAndDevice(c)
		return []rateLimitCheck{
			{Policy: rateLimitPolicy{Name: "auth_ip", Limit: 30, Burst: 10, Window: defaultRateLimitWindow, FailOpen: false}, Key: ip},
			{Policy: rateLimitPolicy{Name: "auth_identity", Limit: 8, Burst: 4, Window: defaultRateLimitWindow, FailOpen: false}, Key: ip + ":" + identity},
			{Policy: rateLimitPolicy{Name: "auth_device", Limit: 20, Burst: 5, Window: defaultRateLimitWindow, FailOpen: false}, Key: ip + ":" + device},
		}
	}

	policy := policyForPath(path, method, cfg)
	return []rateLimitCheck{{Policy: policy, Key: routeRateLimitKey(c, path, method, ip)}}
}

func policyForPath(path, method string, cfg RateLimitConfig) rateLimitPolicy {
	if isAuthWritePath(path) {
		return rateLimitPolicy{Name: "auth_write", Limit: 20, Burst: 10, Window: defaultRateLimitWindow, FailOpen: false}
	}
	if strings.Contains(path, "/admin/") || strings.Contains(path, "/operations/") {
		return rateLimitPolicy{Name: "admin", Limit: 60, Burst: 20, Window: defaultRateLimitWindow, FailOpen: false}
	}
	if isUploadPath(path, method) {
		return rateLimitPolicy{Name: "upload", Limit: 30, Burst: 10, Window: defaultRateLimitWindow, FailOpen: false}
	}
	if action, ok := hocLieuAssetAccessAction(path, method); ok {
		switch action {
		case "launch":
			return rateLimitPolicy{Name: "hoclieu_asset_launch", Limit: 90, Burst: 30, Window: defaultRateLimitWindow, FailOpen: false}
		case "stream":
			return rateLimitPolicy{Name: "hoclieu_asset_stream", Limit: 240, Burst: 60, Window: defaultRateLimitWindow, FailOpen: false}
		case "download":
			return rateLimitPolicy{Name: "hoclieu_asset_download", Limit: 30, Burst: 10, Window: defaultRateLimitWindow, FailOpen: false}
		}
	}
	if isLMSQuizSubmitPath(path, method) {
		return rateLimitPolicy{Name: "lms_quiz_submit", Limit: 12, Burst: 6, Window: defaultRateLimitWindow, FailOpen: false}
	}
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return rateLimitPolicy{Name: "public_read", Limit: maxInt(cfg.RequestsPerMinute*4, 400), Burst: maxInt(cfg.Burst*4, 100), Window: defaultRateLimitWindow, FailOpen: true}
	}
	return basePolicy(cfg, "api_write", false)
}

func basePolicy(cfg RateLimitConfig, name string, failOpen bool) rateLimitPolicy {
	return rateLimitPolicy{
		Name:     name,
		Limit:    cfg.RequestsPerMinute,
		Burst:    cfg.Burst,
		Window:   defaultRateLimitWindow,
		FailOpen: failOpen,
	}
}

func normalizeRateLimitPolicy(policy rateLimitPolicy) rateLimitPolicy {
	if policy.Name == "" {
		policy.Name = "default"
	}
	if policy.Limit <= 0 {
		policy.Limit = 60
	}
	if policy.Window <= 0 {
		policy.Window = defaultRateLimitWindow
	}
	if policy.Burst < 0 {
		policy.Burst = 0
	}
	return policy
}

func effectiveRateLimit(policy rateLimitPolicy) int {
	policy = normalizeRateLimitPolicy(policy)
	return policy.Limit + policy.Burst
}

func observeRateLimitDecision(c *gin.Context, policy rateLimitPolicy, decision rateLimitDecision, elapsed time.Duration) {
	outcome := "allowed"
	if decision.Err != nil {
		outcome = "redis_error"
		if policy.FailOpen {
			outcome = "redis_error_fail_open"
		}
	} else if !decision.Allowed {
		outcome = "blocked"
	}
	backend := decision.Backend
	if backend == "" {
		backend = "unknown"
	}
	route := routeMetricName(c, policy.Name)
	rateLimitRequestsTotal.WithLabelValues(policy.Name, route, backend, outcome).Inc()
	rateLimitDecisionDuration.WithLabelValues(policy.Name, route, backend).Observe(elapsed.Seconds())
}

func writeRateLimited(c *gin.Context, decision rateLimitDecision) {
	if decision.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(maxInt(1, int(decision.RetryAfter.Seconds()))))
	}
	addRateLimitHeaders(c, decision)
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"code":    "ERR_RATE_LIMITED",
			"message": "too many requests; please retry later",
		},
	})
}

func writeRateLimitUnavailable(c *gin.Context, decision rateLimitDecision) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"code":    "ERR_RATE_LIMITER_UNAVAILABLE",
			"message": "rate limiter is unavailable; please retry later",
		},
	})
}

func addRateLimitHeaders(c *gin.Context, decision rateLimitDecision) {
	if decision.Limit > 0 {
		c.Header("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
	}
	if decision.Remaining >= 0 {
		c.Header("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	}
	if decision.ResetAfter > 0 {
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(decision.ResetAfter).Unix(), 10))
	}
}

// IPKeyFunc returns the client IP as the rate limit key.
func IPKeyFunc(c *gin.Context) string {
	return clientIPKey(c)
}

// UserKeyFunc returns the authenticated user ID as the rate limit key.
func UserKeyFunc(c *gin.Context) string {
	return GetUserID(c)
}

func clientIPKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ip := GetRealIP(c.Request.Context()); ip != "" {
		return ip
	}
	return c.ClientIP()
}

func shouldSkipRateLimit(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	path := c.Request.URL.Path
	switch path {
	case "/api/health", "/api/ready", "/metrics", "/swagger", "/reference":
		return true
	default:
		return strings.HasPrefix(path, "/swagger/")
	}
}

func isAuthLoginPath(path string) bool {
	switch path {
	case "/api/auth/login", "/api/lms/auth/login", "/api/auth/google/login", "/api/lms/auth/providers/google":
		return true
	default:
		return false
	}
}

func isAuthWritePath(path string) bool {
	return strings.HasPrefix(path, "/api/auth/") || strings.HasPrefix(path, "/api/lms/auth/")
}

func isUploadPath(path, method string) bool {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return false
	}
	path = strings.ToLower(path)
	return strings.Contains(path, "upload") || strings.Contains(path, "attachment") || strings.Contains(path, "document")
}

func routeRateLimitKey(c *gin.Context, path, method, ip string) string {
	if action, ok := hocLieuAssetAccessAction(path, method); ok {
		return scopedRouteKey(c, ip, "asset:"+routeParamOrSegment(c, path, "assets")+":"+action)
	}
	if isLMSQuizSubmitPath(path, method) {
		return scopedRouteKey(c, ip, "attempt:"+routeParamOrSegment(c, path, "attempts")+":submit")
	}
	return ip
}

func scopedRouteKey(c *gin.Context, ip, suffix string) string {
	userID := ""
	if c != nil && c.Request != nil {
		userID = GetUserID(c.Request.Context())
	}
	if userID == "" {
		userID = "ip:" + ip
	} else {
		userID = "user:" + userID
	}
	return userID + ":" + suffix
}

func hocLieuAssetAccessAction(path, method string) (string, bool) {
	if method != http.MethodGet && method != http.MethodHead {
		return "", false
	}
	parts := splitPath(path)
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "assets" || i+3 != len(parts) {
			continue
		}
		action := parts[i+2]
		switch action {
		case "launch", "stream", "download":
			return action, true
		default:
			return "", false
		}
	}
	return "", false
}

func isLMSQuizSubmitPath(path, method string) bool {
	if method != http.MethodPost {
		return false
	}
	parts := splitPath(path)
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "attempts" && i+3 == len(parts) && parts[i+2] == "submit" {
			return true
		}
	}
	return false
}

func routeParamOrSegment(c *gin.Context, path, segment string) string {
	if c != nil {
		switch segment {
		case "assets":
			if value := c.Param("assetId"); value != "" {
				return value
			}
		case "attempts":
			if value := c.Param("attemptId"); value != "" {
				return value
			}
		}
	}
	parts := splitPath(path)
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == segment {
			return parts[i+1]
		}
	}
	return "unknown"
}

func splitPath(path string) []string {
	raw := strings.Split(strings.ToLower(strings.Trim(path, "/")), "/")
	parts := raw[:0]
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func routePolicyName(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "api"
	}
	return policyForPath(c.Request.URL.Path, c.Request.Method, RateLimitConfig{}).Name
}

func routeFailOpen(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions
}

func routeMetricName(c *gin.Context, fallback string) string {
	if c != nil {
		if fullPath := c.FullPath(); fullPath != "" {
			return fullPath
		}
	}
	return fallback
}

func authIdentityAndDevice(c *gin.Context) (string, string) {
	var payload struct {
		Email             string `json:"email"`
		DeviceID          string `json:"deviceId"`
		DeviceFingerprint string `json:"deviceFingerprint"`
	}
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return "unknown", "unknown"
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, authBodyReadLimit))
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return "unknown", "unknown"
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	_ = json.Unmarshal(body, &payload)

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" {
		email = "unknown"
	}
	device := strings.TrimSpace(payload.DeviceFingerprint)
	if device == "" {
		device = strings.TrimSpace(payload.DeviceID)
	}
	if device == "" {
		device = "unknown"
	}
	return hashRateLimitPart(email), hashRateLimitPart(device)
}

func hashRateLimitPart(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:16])
}

func int64Result(values []interface{}, index int) int64 {
	if index < 0 || index >= len(values) {
		return 0
	}
	switch v := values[index].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

func errRateLimiterUnavailable() error {
	return fmt.Errorf("redis rate limiter unavailable")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
