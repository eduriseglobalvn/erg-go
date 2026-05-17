// Package cache provides a Redis client wrapper with connection pooling,
// pub/sub, distributed locks, and pipeline support.
package cache

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// ErrLockNotAcquired is returned when a distributed lock cannot be acquired.
var ErrLockNotAcquired = errors.New("cache: lock not acquired")

// ErrLockNotHeld is returned when trying to release a lock not held by this caller.
var ErrLockNotHeld = errors.New("cache: lock not held")

// RedisClient wraps a redis.Client with a logger and configuration.
type RedisClient struct {
	client *redis.Client
	cfg    config.RedisConfig
	log    *logger.Logger
}

type redisInternalLogger struct {
	log  *logger.Logger
	mu   sync.Mutex
	last map[string]time.Time
}

// RedisOption configures a RedisClient.
type RedisOption func(*RedisClient)

// WithRedisLogger sets the logger for the Redis client.
func WithRedisLogger(log *logger.Logger) RedisOption {
	return func(r *RedisClient) {
		r.log = log
	}
}

// NewRedisClient creates a new Redis client from the given configuration.
// It configures connection pooling, timeouts, and retries.
func NewRedisClient(ctx context.Context, cfg config.RedisConfig, opts ...RedisOption) (*RedisClient, error) {
	r := &RedisClient{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(r)
	}
	redis.SetLogger(newRedisInternalLogger(r.log))

	options := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
	}
	if cfg.TLS {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.Host,
		}
	}

	client := redis.NewClient(options)

	// Verify connectivity.
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	r.client = client
	r.log.Info().Str("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)).Msg("redis connected")

	return r, nil
}

// Client returns the underlying redis.Client.
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// Ping checks Redis connectivity.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close shuts down the Redis client.
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Set stores a key-value pair with an optional TTL.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("redis: set %s: %w", key, err)
	}
	return nil
}

// Get retrieves a value by key. Returns redis.Nil if the key does not exist.
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("redis: get %s: %w", key, err)
	}
	return val, nil
}

// Del deletes one or more keys.
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	err := r.client.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("redis: del %v: %w", keys, err)
	}
	return nil
}

// SetNX sets a key only if it does not exist, with a TTL. Used for distributed locking.
func (r *RedisClient) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis: setnx %s: %w", key, err)
	}
	return ok, nil
}

// Incr atomically increments a key. Returns the new value or an error.
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	val, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: incr %s: %w", key, err)
	}
	return val, nil
}

// Expire sets a TTL on a key.
func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ok, err := r.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		return fmt.Errorf("redis: expire %s: %w", key, err)
	}
	if !ok {
		return fmt.Errorf("redis: expire %s: key does not exist", key)
	}
	return nil
}

// TTL returns the remaining TTL for a key.
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: ttl %s: %w", key, err)
	}
	return ttl, nil
}

// Exists checks if a key exists.
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	n, err := r.client.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: exists %v: %w", keys, err)
	}
	return n, nil
}

// Publish sends a message to a Redis channel.
func (r *RedisClient) Publish(ctx context.Context, channel string, message interface{}) error {
	err := r.client.Publish(ctx, channel, message).Err()
	if err != nil {
		return fmt.Errorf("redis: publish %s: %w", channel, err)
	}
	return nil
}

// Subscribe listens for messages on one or more channels.
// The cancel function stops the subscription.
func (r *RedisClient) Subscribe(ctx context.Context, channels ...string) (*redis.PubSub, func()) {
	pubsub := r.client.Subscribe(ctx, channels...)
	return pubsub, func() { _ = pubsub.Close() }
}

// PSubscribe listens for messages matching a pattern. The cancel function stops it.
func (r *RedisClient) PSubscribe(ctx context.Context, pattern string) (*redis.PubSub, func()) {
	pubsub := r.client.PSubscribe(ctx, pattern)
	return pubsub, func() { _ = pubsub.Close() }
}

// Pipeline returns a new pipeline for batching commands.
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// PipelineCmd represents a single pipelined command.
type PipelineCmd = redis.StringCmd

// DistributedLock represents a Redis-based distributed mutex.
type DistributedLock struct {
	client *redis.Client
	key    string
	value  string
	ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock with the given key and TTL.
// It uses SETNX semantics with a unique value to ensure only the owner can release.
func (r *RedisClient) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*DistributedLock, error) {
	value, err := generateLockValue()
	if err != nil {
		return nil, fmt.Errorf("cache: generate lock value: %w", err)
	}

	ok, err := r.SetNX(ctx, key, value, ttl)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}

	return &DistributedLock{
		client: r.client,
		key:    key,
		value:  value,
		ttl:    ttl,
	}, nil
}

// Release releases the distributed lock. Only the owner (matching value) can release it.
func (l *DistributedLock) Release(ctx context.Context) error {
	// Use Lua script to atomically check-and-delete to avoid race conditions.
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	result, err := script.Run(ctx, l.client, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("cache: release lock %s: %w", l.key, err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// Extend extends the TTL of a held lock.
func (l *DistributedLock) Extend(ctx context.Context, ttl time.Duration) error {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	result, err := script.Run(ctx, l.client, []string{l.key}, l.value, ttl.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("cache: extend lock %s: %w", l.key, err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// generateLockValue creates a unique random value for lock ownership.
func generateLockValue() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newRedisInternalLogger(log *logger.Logger) *redisInternalLogger {
	if log == nil {
		log = logger.NoOp()
	}
	return &redisInternalLogger{
		log:  log,
		last: make(map[string]time.Time),
	}
}

func (l *redisInternalLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if strings.Contains(msg, "discarding bad PubSub connection") && !l.shouldLog(msg, 30*time.Second) {
		return
	}
	l.log.DebugContext(ctx).
		Str("component", "go-redis").
		Msg(msg)
}

func (l *redisInternalLogger) shouldLog(key string, interval time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if last, ok := l.last[key]; ok && now.Sub(last) < interval {
		return false
	}
	l.last[key] = now
	return true
}
