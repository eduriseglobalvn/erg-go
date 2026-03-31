package config

import (
	"os"
	"testing"
	"time"
)

func TestNewDefault(t *testing.T) {
	cfg := NewDefault()
	if cfg == nil {
		t.Fatal("NewDefault returned nil")
	}
	if cfg.App.Name != "erg-service" {
		t.Errorf("expected app name 'erg-service', got %q", cfg.App.Name)
	}
	if cfg.App.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.App.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Logging.Level)
	}
	if cfg.Scraper.MinDelay != 3*time.Second {
		t.Errorf("expected min delay 3s, got %v", cfg.Scraper.MinDelay)
	}
	if len(cfg.Scraper.UserAgents) == 0 {
		t.Error("expected at least one user agent")
	}
}

func TestLoader_InjectSecrets(t *testing.T) {
	os.Setenv("SECRET_DATABASE__PASSWORD", "super-secret")
	defer os.Unsetenv("SECRET_DATABASE__PASSWORD")

	l := NewLoader()
	v := globalViper
	l.injectSecrets(v)

	// After injectSecrets, a key like "database.password" should be set.
	if got := v.GetString("database.password"); got != "super-secret" {
		t.Errorf("expected secret 'super-secret', got %q", got)
	}
}

func TestLoader_Load(t *testing.T) {
	// Create a temporary config file.
	tmp := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(tmp, []byte(`
app:
  name: test-service
  port: 9090
  env: testing
database:
  host: db.test
  port: 5432
redis:
  host: redis.test
  port: 6380
`), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmp[:len(tmp)-len("/config.yaml")]), WithFileName("config"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Name != "test-service" {
		t.Errorf("expected app name 'test-service', got %q", cfg.App.Name)
	}
	if cfg.App.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.App.Port)
	}
	if cfg.Database.Host != "db.test" {
		t.Errorf("expected db host 'db.test', got %q", cfg.Database.Host)
	}
	if cfg.Redis.Host != "redis.test" {
		t.Errorf("expected redis host 'redis.test', got %q", cfg.Redis.Host)
	}
}

func TestGetString_Helper(t *testing.T) {
	// Seed global cfg.
	globalCfgMu.Lock()
	globalCfg = NewDefault()
	globalCfgMu.Unlock()

	defer func() {
		globalCfgMu.Lock()
		globalCfg = nil
		globalCfgMu.Unlock()
	}()

	if got := GetString("app.name", "fallback"); got != "erg-service" {
		t.Errorf("GetString returned %q, want 'erg-service'", got)
	}
	if got := GetString("nonexistent.key", "fallback"); got != "fallback" {
		t.Errorf("GetString returned %q for missing key, want 'fallback'", got)
	}
}

func TestGetInt_Helper(t *testing.T) {
	globalCfgMu.Lock()
	globalCfg = NewDefault()
	globalCfgMu.Unlock()

	if got := GetInt("app.port", 0); got != 8080 {
		t.Errorf("GetInt returned %d, want 8080", got)
	}
}

func TestGetBool_Helper(t *testing.T) {
	globalCfgMu.Lock()
	globalCfg = NewDefault()
	globalCfgMu.Unlock()

	if got := GetBool("telemetry.enabled", false); got != true {
		t.Errorf("GetBool returned %v, want true", got)
	}
}
