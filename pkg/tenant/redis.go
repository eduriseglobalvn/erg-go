// Package tenant provides Redis key wrappers with per-tenant key isolation.
package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"erg.ninja/pkg/cache"
)

// TenantRedis wraps a *cache.RedisClient with automatic tenant key prefixing.
// All key operations are namespaced under "tenant:{tenant_id}:".
//
// Key namespace convention:
//
//	tenant:{tenant_id}:{module}:{entity}:{id}
//
// Examples:
//
//	tenant:acme:crawler:crawl_histories:url_hash_abc123
//	tenant:acme:trending:topics:page_1
//	tenant:acme:notifications:queue:user_123
type TenantRedis struct {
	*cache.RedisClient
	tenantID string
}

// NewTenantRedis returns a TenantRedis that prefixes all keys with the given tenant ID.
// The returned instance is safe to use concurrently.
func NewTenantRedis(r *cache.RedisClient, tenantID string) *TenantRedis {
	if tenantID == "" {
		tenantID = "default"
	}
	return &TenantRedis{RedisClient: r, tenantID: tenantID}
}

// NewTenantRedisFromContext returns a TenantRedis for the tenant ID currently in ctx.
func NewTenantRedisFromContext(r *cache.RedisClient, ctx context.Context) *TenantRedis {
	return NewTenantRedis(r, FromContext(ctx))
}

// prefixKey returns the full tenant-scoped key.
func (t *TenantRedis) prefixKey(key string) string {
	return fmt.Sprintf("tenant:%s:%s", t.tenantID, key)
}

// Get retrieves the value for key, scoped to the tenant.
func (t *TenantRedis) Get(ctx context.Context, key string) (string, error) {
	return t.RedisClient.Get(ctx, t.prefixKey(key))
}

// Set stores value for key with optional TTL, scoped to the tenant.
func (t *TenantRedis) Set(ctx context.Context, key string, value interface{}) error {
	return t.RedisClient.Set(ctx, t.prefixKey(key), value, 0)
}

// SetTTL stores value for key with a time.Duration TTL, scoped to the tenant.
func (t *TenantRedis) SetTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return t.RedisClient.Set(ctx, t.prefixKey(key), value, ttl)
}

// Del deletes one or more tenant-scoped keys.
// Accepts bare key names; the tenant prefix is applied automatically.
func (t *TenantRedis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = t.prefixKey(k)
	}
	return t.RedisClient.Del(ctx, full...)
}

// SetNX sets a key only if it does not exist, scoped to the tenant.
func (t *TenantRedis) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return t.RedisClient.SetNX(ctx, t.prefixKey(key), value, ttl)
}

// Incr atomically increments a tenant-scoped key.
func (t *TenantRedis) Incr(ctx context.Context, key string) (int64, error) {
	return t.RedisClient.Incr(ctx, t.prefixKey(key))
}

// Expire sets a TTL on a tenant-scoped key.
func (t *TenantRedis) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return t.RedisClient.Expire(ctx, t.prefixKey(key), ttl)
}

// Exists checks if a tenant-scoped key exists.
func (t *TenantRedis) Exists(ctx context.Context, key string) (int64, error) {
	return t.RedisClient.Exists(ctx, t.prefixKey(key))
}

// TenantID returns the tenant ID for this wrapper.
func (t *TenantRedis) TenantID() string {
	return t.tenantID
}

// Publish sends a message to a channel namespaced under the tenant.
func (t *TenantRedis) Publish(ctx context.Context, channel string, message interface{}) error {
	return t.RedisClient.Publish(ctx, t.prefixKey(channel), message)
}

// Subscribe subscribes to a tenant-scoped channel.
// Returns the pubsub and a cancel function.
func (t *TenantRedis) Subscribe(ctx context.Context, channel string) (*redis.PubSub, func()) {
	return t.RedisClient.Subscribe(ctx, t.prefixKey(channel))
}

var _ = (*redis.PubSub)(nil) // suppress unused import warning when pubsub helpers are added

// FullKey returns the fully-prefixed Redis key for a given bare key.
func (t *TenantRedis) FullKey(key string) string {
	return t.prefixKey(key)
}

// Prefix returns the tenant key prefix for use in KEYS/SCAN pattern matching.
func (t *TenantRedis) Prefix() string {
	return fmt.Sprintf("tenant:%s:", t.tenantID)
}

// TenantAwarePubSub is a tenant-scoped pub/sub wrapper.
type TenantAwarePubSub struct {
	channel  string
	tenantID string
}

// NewTenantPubSub returns a TenantAwarePubSub for the given channel and tenant.
func NewTenantPubSub(tenantID, channel string) *TenantAwarePubSub {
	if tenantID == "" {
		tenantID = "default"
	}
	return &TenantAwarePubSub{channel: channel, tenantID: tenantID}
}

// Channel returns the fully-qualified tenant-scoped channel name.
func (p *TenantAwarePubSub) Channel() string {
	return fmt.Sprintf("tenant:%s:%s", p.tenantID, p.channel)
}
