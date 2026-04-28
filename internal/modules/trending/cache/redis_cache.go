package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appcache "erg.ninja/pkg/cache"
)

const (
	topicsKey = "trending:topics"
	newsKey   = "trending:news"
	feedKey   = "trending:urls"
)

// RedisCache wraps Redis access for the trending module.
type RedisCache struct {
	client *appcache.RedisClient
	ttl    time.Duration
}

// NewRedisCache creates a RedisCache.
func NewRedisCache(client *appcache.RedisClient, ttl time.Duration) *RedisCache {
	return &RedisCache{client: client, ttl: ttl}
}

// SetJSON caches a JSON-serializable value.
func (c *RedisCache) SetJSON(ctx context.Context, key string, value any) error {
	if c == nil || c.client == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("trending cache marshal %s: %w", key, err)
	}
	return c.client.Set(ctx, key, raw, c.ttl)
}

// GetJSON reads and decodes a cached JSON value.
func (c *RedisCache) GetJSON(ctx context.Context, key string, out any) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	raw, err := c.client.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if raw == "" {
		return false, nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, fmt.Errorf("trending cache unmarshal %s: %w", key, err)
	}
	return true, nil
}

// StoreDiscoveryFeed overwrites the Redis discovery list for crawler consumption.
func (c *RedisCache) StoreDiscoveryFeed(ctx context.Context, urls []string, maxItems int64) error {
	if c == nil || c.client == nil {
		return nil
	}
	pipe := c.client.Client().Pipeline()
	pipe.Del(ctx, feedKey)
	if len(urls) > 0 {
		values := make([]any, 0, len(urls))
		for _, u := range urls {
			values = append(values, u)
		}
		pipe.RPush(ctx, feedKey, values...)
		pipe.LTrim(ctx, feedKey, 0, maxItems-1)
	}
	pipe.Expire(ctx, feedKey, c.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetDiscoveryFeed returns the latest discovery URLs from Redis.
func (c *RedisCache) GetDiscoveryFeed(ctx context.Context, limit int64) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	return c.client.Client().LRange(ctx, feedKey, 0, limit-1).Result()
}

func TopicsKey() string { return topicsKey }
func NewsKey() string   { return newsKey }
