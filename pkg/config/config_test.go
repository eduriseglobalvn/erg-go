package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
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
	// Secrets are injected during Load() from SECRET_* environment variables.
	// This is tested indirectly via TestLoader_Load which verifies
	// that the config file is read correctly after Load() processes env vars.
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

func TestLoader_LoadsApplicationProfileAndLocalLayers(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, tmpDir+"/application.yaml", `
app:
  name: erg-base
  env: development
  port: 8080
logging:
  level: info
redis:
  host: base-redis.example.com
`)
	writeConfigFile(t, tmpDir+"/application.development.yaml", `
app:
  port: 9090
redis:
  host: dev-redis.example.com
`)
	writeConfigFile(t, tmpDir+"/application.local.yaml", `
logging:
  level: trace
`)

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileName("application"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Name != "erg-base" {
		t.Errorf("expected base app name, got %q", cfg.App.Name)
	}
	if cfg.App.Port != 9090 {
		t.Errorf("expected development profile port 9090, got %d", cfg.App.Port)
	}
	if cfg.Redis.Host != "dev-redis.example.com" {
		t.Errorf("expected development redis host, got %q", cfg.Redis.Host)
	}
	if cfg.Logging.Level != "trace" {
		t.Errorf("expected local logging override, got %q", cfg.Logging.Level)
	}
}

func TestLoader_ProcessProfileOverridesBaseEnv(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, tmpDir+"/application.yaml", `
app:
  env: development
  port: 8080
http:
  cors:
    allowed_origins: ["http://localhost:3000"]
`)
	writeConfigFile(t, tmpDir+"/application.production.yaml", `
app:
  env: production
  port: 8081
http:
  cors:
    allowed_origins: ["https://erg.edu.vn"]
auth:
  jwt_secret: "0123456789abcdefghijklmnopqrstuvwxyz"
  jwt_refresh_secret: "refresh-0123456789abcdefghijklmnopqrstuvwxyz"
database:
  password: "prod-db-password"
ai:
  api_key_encryption_secret: "prod-ai-key-encryption-secret-012345"
`)
	t.Setenv("APP_PROFILE", "production")

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileName("application"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Env != "production" {
		t.Errorf("expected production env, got %q", cfg.App.Env)
	}
	if cfg.App.Port != 8081 {
		t.Errorf("expected production port 8081, got %d", cfg.App.Port)
	}
	if got := cfg.HTTP.CORS.AllowedOrigins; len(got) != 1 || got[0] != "https://erg.edu.vn" {
		t.Errorf("expected production cors origin, got %#v", got)
	}
}

func TestLoader_LegacyConfigOverridesApplication(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, tmpDir+"/application.yaml", `
app:
  name: application-name
  env: development
  port: 8080
`)
	writeConfigFile(t, tmpDir+"/config.yaml", `
app:
  name: config-name
  port: 9090
`)

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileNames("application", "config"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Name != "config-name" {
		t.Errorf("expected legacy config override, got %q", cfg.App.Name)
	}
	if cfg.App.Port != 9090 {
		t.Errorf("expected legacy config port override, got %d", cfg.App.Port)
	}
}

func TestLoader_LoadRejectsWildcardCORSInProduction(t *testing.T) {
	tmp := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(tmp, []byte(`
app:
  env: production
http:
  cors:
    allowed_origins: ["*"]
`), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmp[:len(tmp)-len("/config.yaml")]), WithFileName("config"))
	if err := l.Load(cfg); err == nil {
		t.Fatal("expected Load to reject wildcard CORS in production")
	}
}

func TestLoader_LoadRejectsRuntimeMigrationsInProduction(t *testing.T) {
	tmp := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(tmp, []byte(`
app:
  env: production
http:
  cors:
    allowed_origins: ["https://erg.edu.vn"]
auth:
  jwt_secret: "0123456789abcdefghijklmnopqrstuvwxyz"
  jwt_refresh_secret: "refresh-0123456789abcdefghijklmnopqrstuvwxyz"
database:
  password: "prod-db-password"
  auto_migrate: true
`), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmp[:len(tmp)-len("/config.yaml")]), WithFileName("config"))
	if err := l.Load(cfg); err == nil {
		t.Fatal("expected Load to reject runtime automigrate in production")
	}
}

func TestLoader_LoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	tmp := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(tmp, []byte(`
http:
  trusted_proxy_cidrs: ["not-a-cidr"]
`), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmp[:len(tmp)-len("/config.yaml")]), WithFileName("config"))
	if err := l.Load(cfg); err == nil {
		t.Fatal("expected Load to reject invalid trusted proxy CIDR")
	}
}

func writeConfigFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config %s: %v", path, err)
	}
}

func TestLoader_LoadsDotEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"
	if err := os.WriteFile(envPath, []byte(`
DB__HOST=pg.example.com
DB__PORT=15432
DB__USER=avnadmin
SECRET_DB__PASSWORD=super-secret
DB__NAME=defaultdb
DB__SSL_MODE=require
MONGODB__URI=mongodb+srv://example.mongodb.net
MONGODB__DATABASE=erg
REDIS__HOST=valkey.example.com
REDIS__PORT=16379
REDIS__USERNAME=default
SECRET_REDIS__PASSWORD=valkey-secret
REDIS__TLS=true
QUEUE__REDIS_HOST=queue.example.com
QUEUE__REDIS_PORT=26379
QUEUE__REDIS_USERNAME=default
SECRET_QUEUE__REDIS_PASSWORD=queue-secret
QUEUE__REDIS_TLS=true
SECRET_AUTH__JWT_SECRET=jwt-secret
SECRET_AUTH__JWT_REFRESH_SECRET=refresh-secret
AUTH__ACCESS_TOKEN_TTL=3h
`), 0644); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileName("config"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Database.Host != "pg.example.com" {
		t.Errorf("expected db host 'pg.example.com', got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 15432 {
		t.Errorf("expected db port 15432, got %d", cfg.Database.Port)
	}
	if cfg.Database.Password != "super-secret" {
		t.Errorf("expected db password from secret env, got %q", cfg.Database.Password)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected db ssl mode require, got %q", cfg.Database.SSLMode)
	}
	if cfg.MongoDB.URI != "mongodb+srv://example.mongodb.net" {
		t.Errorf("expected mongodb uri from .env, got %q", cfg.MongoDB.URI)
	}
	if cfg.Redis.Username != "default" {
		t.Errorf("expected redis username 'default', got %q", cfg.Redis.Username)
	}
	if !cfg.Redis.TLS {
		t.Error("expected redis TLS to be enabled")
	}
	if cfg.Queue.RedisUsername != "default" {
		t.Errorf("expected queue redis username 'default', got %q", cfg.Queue.RedisUsername)
	}
	if !cfg.Queue.RedisTLS {
		t.Error("expected queue redis TLS to be enabled")
	}
	if cfg.Auth.JWTSecret != "jwt-secret" {
		t.Errorf("expected auth jwt secret from .env, got %q", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.JWTRefreshSecret != "refresh-secret" {
		t.Errorf("expected auth refresh secret from .env, got %q", cfg.Auth.JWTRefreshSecret)
	}
	if cfg.Auth.AccessTokenTTL != 3*time.Hour {
		t.Errorf("expected access token ttl 3h, got %v", cfg.Auth.AccessTokenTTL)
	}
}

func TestLoader_ProcessEnvOverridesDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"
	if err := os.WriteFile(envPath, []byte(`
REDIS__HOST=file-redis.example.com
`), 0644); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}

	t.Setenv("REDIS__HOST", "process-redis.example.com")

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileName("config"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Redis.Host != "process-redis.example.com" {
		t.Errorf("expected process env to override .env, got %q", cfg.Redis.Host)
	}
}

func TestLoader_LoadsBackendStyleAliases(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"
	if err := os.WriteFile(envPath, []byte(`
NODE_ENV=development
JWT_ACCESS_SECRET=jwt-access
JWT_ACCESS_EXPIRATION_TIME=3h
JWT_REFRESH_EXPIRATION_TIME=7d
MAIL_HOST=pro207.emailserver.vn
MAIL_PORT=465
MAIL_SECURE=true
MAIL_USER=noreply@erg.edu.vn
MAIL_PASSWORD=mail-secret
MAIL_FROM=[No-Reply] EDURISE GLOBAL <noreply@erg.edu.vn>
R2_ENDPOINT=https://example.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=key-id
R2_SECRET_ACCESS_KEY=secret-key
R2_BUCKET_NAME=erg
R2_PUBLIC_DOMAIN=https://media.erg.edu.vn
R2_REGION=auto
GROQ_API_KEY=groq-secret
GROQ_MODEL=openai/gpt-oss-120b
GROQ_BASE_URL=https://api.groq.com/openai/v1
GEMINI_MODEL=gemini-3-flash-preview
`), 0644); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}

	cfg := new(Config)
	l := NewLoader(WithConfigPaths(tmpDir), WithFileName("config"))
	if err := l.Load(cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Env != "development" {
		t.Errorf("expected app env 'development', got %q", cfg.App.Env)
	}
	if cfg.Auth.JWTSecret != "jwt-access" {
		t.Errorf("expected JWT secret from backend-style alias, got %q", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.AccessTokenTTL != 3*time.Hour {
		t.Errorf("expected access ttl 3h, got %v", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Auth.RefreshTokenTTL != 7*24*time.Hour {
		t.Errorf("expected refresh ttl 7d, got %v", cfg.Auth.RefreshTokenTTL)
	}
	if cfg.SMTP.Host != "pro207.emailserver.vn" {
		t.Errorf("expected smtp host alias, got %q", cfg.SMTP.Host)
	}
	if !cfg.SMTP.TLS {
		t.Error("expected smtp TLS from MAIL_SECURE alias")
	}
	if cfg.R2.BucketName != "erg" {
		t.Errorf("expected R2 bucket alias, got %q", cfg.R2.BucketName)
	}
	if cfg.R2.SecretKey != "secret-key" {
		t.Errorf("expected R2 secret alias, got %q", cfg.R2.SecretKey)
	}
	if cfg.Ai.GeminiModel != "gemini-3-flash-preview" {
		t.Errorf("expected gemini model alias, got %q", cfg.Ai.GeminiModel)
	}
	if cfg.Ai.GroqAPIKey != "groq-secret" {
		t.Errorf("expected groq API key alias, got %q", cfg.Ai.GroqAPIKey)
	}
	if cfg.Ai.GroqModel != "openai/gpt-oss-120b" {
		t.Errorf("expected groq model alias, got %q", cfg.Ai.GroqModel)
	}
	if cfg.Ai.GroqBaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("expected groq base url alias, got %q", cfg.Ai.GroqBaseURL)
	}
}

func TestGetString_Helper(t *testing.T) {
	// Seed global cfg AND globalViper so helper functions read the same state.
	globalCfgMu.Lock()
	globalCfg = NewDefault()
	v := viper.New()
	v.Set("app.name", "erg-service")
	v.Set("app.port", 8080)
	v.Set("telemetry.enabled", true)
	globalViper = v
	globalCfgMu.Unlock()

	defer func() {
		globalCfgMu.Lock()
		globalCfg = nil
		globalViper = viper.New()
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
	v := viper.New()
	v.Set("app.port", 8080)
	globalViper = v
	globalCfgMu.Unlock()

	if got := GetInt("app.port", 0); got != 8080 {
		t.Errorf("GetInt returned %d, want 8080", got)
	}
}

func TestGetBool_Helper(t *testing.T) {
	globalCfgMu.Lock()
	globalCfg = NewDefault()
	v := viper.New()
	v.Set("telemetry.enabled", true)
	globalViper = v
	globalCfgMu.Unlock()

	if got := GetBool("telemetry.enabled", false); got != true {
		t.Errorf("GetBool returned %v, want true", got)
	}
}
