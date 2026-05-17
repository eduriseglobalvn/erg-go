// Package cache provides a shared Redis cache interface used across bot-service packages.
package cache

import (
	"context"
	"time"
)

// RedisCache is the minimal Redis interface used by bot-service internal packages.
// It avoids import cycles by not referencing the pkg/cache package directly.
type RedisCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisLock provides distributed locking via Redis.
type RedisLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}
