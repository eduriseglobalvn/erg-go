package cache

import (
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestRedisClientDefaults(t *testing.T) {
	cfg := config.NewDefault().Redis
	if cfg.Host == "" {
		t.Error("Redis Host should not be empty")
	}
	if cfg.PoolSize == 0 {
		t.Error("PoolSize should not be zero")
	}
	if cfg.DialTimeout == 0 {
		t.Error("DialTimeout should not be zero")
	}
}

func TestRedisConfigDefaults(t *testing.T) {
	cfg := config.NewDefault().Redis
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.MinIdleConns == 0 {
		t.Error("MinIdleConns should not be zero")
	}
}

func TestGenerateLockValue(t *testing.T) {
	v1, err := generateLockValue()
	if err != nil {
		t.Fatalf("generateLockValue: %v", err)
	}
	if len(v1) != 64 {
		t.Errorf("lock value length = %d, want 64 (hex of 32 bytes)", len(v1))
	}

	v2, err := generateLockValue()
	if err != nil {
		t.Fatalf("generateLockValue second call: %v", err)
	}
	if v1 == v2 {
		t.Error("two lock values should be unique")
	}
}

func TestDistributedLockStruct(t *testing.T) {
	l := &DistributedLock{
		key:   "test-lock",
		value: "unique-owner-id",
		ttl:   10 * time.Second,
	}
	if l.key != "test-lock" {
		t.Errorf("key = %q, want 'test-lock'", l.key)
	}
	if l.value != "unique-owner-id" {
		t.Errorf("value = %q, want 'unique-owner-id'", l.value)
	}
}

func TestErrLockNotAcquired(t *testing.T) {
	if ErrLockNotAcquired.Error() == "" {
		t.Error("ErrLockNotAcquired should have an error message")
	}
}

func TestErrLockNotHeld(t *testing.T) {
	if ErrLockNotHeld.Error() == "" {
		t.Error("ErrLockNotHeld should have an error message")
	}
}
