// Package config provides Viper-based configuration loading for all services.
// It supports YAML files, environment variable overrides, and secret injection.
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration type. Services embed this or a subset of
// its fields and call Load to populate it from config.yaml / environment.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	MongoDB   MongoDBConfig   `mapstructure:"mongodb"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Scraper   ScraperConfig   `mapstructure:"scraper"`
	Ai        AiConfig        `mapstructure:"ai"`
	Auth      AuthConfig      `mapstructure:"auth"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name         string        `mapstructure:"name"`
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Env          string        `mapstructure:"env"` // "development", "production"
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	ShutdownTime time.Duration `mapstructure:"shutdown_time"`
}

// DatabaseConfig holds MySQL/PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// MongoDBConfig holds MongoDB connection settings.
type MongoDBConfig struct {
	URI                    string        `mapstructure:"uri"`
	Database               string        `mapstructure:"database"`
	AuthSource             string        `mapstructure:"auth_source"`
	User                   string        `mapstructure:"user"`
	Password               string        `mapstructure:"password"`
	ReplicaSet             string        `mapstructure:"replica_set"`
	MaxPoolSize            uint64        `mapstructure:"max_pool_size"`
	MinPoolSize            uint64        `mapstructure:"min_pool_size"`
	ServerSelectionTimeout time.Duration `mapstructure:"server_selection_timeout"`
	ConnectTimeout         time.Duration `mapstructure:"connect_timeout"`
	SocketTimeout          time.Duration `mapstructure:"socket_timeout"`
	ReadPreference         string        `mapstructure:"read_preference"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
}

// QueueConfig holds Asynq queue settings.
type QueueConfig struct {
	RedisHost     string        `mapstructure:"redis_host"`
	RedisPort     int           `mapstructure:"redis_port"`
	RedisPassword string        `mapstructure:"redis_password"`
	RedisDB       int           `mapstructure:"redis_db"`
	Concurrency   int           `mapstructure:"concurrency"`
	RetryDelay    time.Duration `mapstructure:"retry_delay"`
	MaxRetry      int           `mapstructure:"max_retry"`
	QueueName     string        `mapstructure:"queue_name"`
	DLQQueueName  string        `mapstructure:"dlq_queue_name"`
	RetryBackoff  bool          `mapstructure:"retry_backoff"`
	IsServer      bool          `mapstructure:"is_server"`
}

// TelemetryConfig holds OpenTelemetry and Prometheus settings.
type TelemetryConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	Environment    string `mapstructure:"environment"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
}

// ScraperConfig holds web scraper settings.
type ScraperConfig struct {
	UserAgents          []string      `mapstructure:"user_agents"`
	ProxyURLs           []string      `mapstructure:"proxy_urls"`
	MinDelay            time.Duration `mapstructure:"min_delay"`
	MaxDelay            time.Duration `mapstructure:"max_delay"`
	Timeout             time.Duration `mapstructure:"timeout"`
	MaxResponseSize     int64         `mapstructure:"max_response_size"`
	RespectRobotsTxt    bool          `mapstructure:"respect_robots_txt"`
	MaxRetries          int           `mapstructure:"max_retries"`
	BlockStatusCodes    []int         `mapstructure:"block_status_codes"`
	BlockPatterns       []string      `mapstructure:"block_patterns"`
	ProxyRotateInterval time.Duration `mapstructure:"proxy_rotate_interval"`
}

// AiConfig holds AI service settings.
type AiConfig struct {
	GeminiAPIKey  string        `mapstructure:"gemini_api_key"`
	GeminiModel   string        `mapstructure:"gemini_model"`
	GeminiTimeout time.Duration `mapstructure:"gemini_timeout"`
	CacheTTL      time.Duration `mapstructure:"cache_ttl"`
	BatchSize    int           `mapstructure:"batch_size"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret     string   `mapstructure:"jwt_secret"`
	JWTPublicKey  string   `mapstructure:"jwt_public_key"`
	JWTIssuer     string   `mapstructure:"jwt_issuer"`
	JWTAlgorithms []string `mapstructure:"jwt_algorithms"`
	BearerPrefix  string   `mapstructure:"bearer_prefix"`
	SkipPaths     []string `mapstructure:"skip_paths"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Host            string           `mapstructure:"host"`
	Port            int              `mapstructure:"port"`
	ReadTimeout     time.Duration    `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration    `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration    `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration    `mapstructure:"shutdown_timeout"`
	RateLimit       RateLimitConfig `mapstructure:"rate_limit"`
	CORS            CORSConfig       `mapstructure:"cors"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute"`
	Burst             int  `mapstructure:"burst"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int       `mapstructure:"max_age"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	TimeFormat string `mapstructure:"time_format"`
}

var (
	globalViper = viper.New()
	globalCfg   *Config
	globalCfgMu sync.RWMutex
)

const secretPrefix = "SECRET_"

// Loader encapsulates Viper configuration options.
type Loader struct {
	configPaths []string
	envPrefix   string
	fileName    string
}

// LoaderOption applies an option to the Loader.
type LoaderOption func(*Loader)

// WithConfigPaths adds directory paths to search for config files.
func WithConfigPaths(paths ...string) LoaderOption {
	return func(l *Loader) {
		l.configPaths = append(l.configPaths, paths...)
	}
}

// WithEnvPrefix sets the environment variable prefix.
func WithEnvPrefix(prefix string) LoaderOption {
	return func(l *Loader) {
		l.envPrefix = prefix
	}
}

// WithFileName sets the config file name without extension (default: "config").
func WithFileName(name string) LoaderOption {
	return func(l *Loader) {
		l.fileName = name
	}
}

// NewLoader creates a new config loader with sensible defaults.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		configPaths: []string{"."},
		envPrefix:   "",
		fileName:    "config",
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Load reads configuration from YAML files and environment variables into out.
func (l *Loader) Load(out interface{}) error {
	v := viper.New()

	for _, p := range l.configPaths {
		v.AddConfigPath(p)
	}
	v.SetConfigName(l.fileName)
	v.SetConfigType("yaml")

	v.SetEnvPrefix(l.envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Inject secrets from environment variables with SECRET_ prefix.
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, secretPrefix) {
			continue
		}
		rest := strings.TrimPrefix(e, secretPrefix)
		parts := strings.SplitN(rest, "=", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(parts[0], "_", "."))
		v.Set(key, parts[1])
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("config: read config file: %w", err)
		}
	}

	if err := v.Unmarshal(out); err != nil {
		return fmt.Errorf("config: unmarshal into struct: %w", err)
	}

	globalCfgMu.Lock()
	globalCfg = out.(*Config)
	globalCfgMu.Unlock()

	return nil
}

// Default returns the globally loaded configuration, if any.
func Default() *Config {
	globalCfgMu.RLock()
	defer globalCfgMu.RUnlock()
	return globalCfg
}

// GetString returns the string value for key, or def if not set.
func GetString(key string, def string) string {
	globalCfgMu.RLock()
	defer globalCfgMu.RUnlock()
	if globalCfg == nil {
		return def
	}
	val := globalViper.GetString(key)
	if val == "" {
		return def
	}
	return val
}

// GetInt returns the int value for key, or def if not set or invalid.
func GetInt(key string, def int) int {
	if !globalViper.IsSet(key) {
		return def
	}
	return globalViper.GetInt(key)
}

// GetDuration returns the duration value for key, or def if not set.
func GetDuration(key string, def time.Duration) time.Duration {
	if !globalViper.IsSet(key) {
		return def
	}
	return globalViper.GetDuration(key)
}

// GetBool returns the bool value for key, or def if not set.
func GetBool(key string, def bool) bool {
	if !globalViper.IsSet(key) {
		return def
	}
	return globalViper.GetBool(key)
}

// Unmarshal unmarshals a subset of config into the provided struct.
func Unmarshal(prefix string, out interface{}) error {
	if err := globalViper.UnmarshalKey(prefix, out); err != nil {
		return fmt.Errorf("config: unmarshal prefix %q: %w", prefix, err)
	}
	return nil
}

// NewDefault returns a Config pre-populated with development defaults.
func NewDefault() *Config {
	return &Config{
		App: AppConfig{
			Name:         "erg-service",
			Host:         "0.0.0.0",
			Port:         8080,
			Env:          "development",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			ShutdownTime: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Name:            "erg",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		MongoDB: MongoDBConfig{
			URI:                    "mongodb://localhost:27017",
			Database:               "erg",
			MaxPoolSize:            100,
			MinPoolSize:            10,
			ServerSelectionTimeout: 10 * time.Second,
			ConnectTimeout:         10 * time.Second,
			ReadPreference:          "secondaryPreferred",
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			DB:           0,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
		},
		Queue: QueueConfig{
			Concurrency: 10,
			RetryDelay:  10 * time.Second,
			MaxRetry:    3,
			RetryBackoff: true,
			QueueName:    "default",
			DLQQueueName: "erg-dlq",
		},
		Telemetry: TelemetryConfig{
			Enabled:        true,
			ServiceName:    "erg-service",
			ServiceVersion: "0.1.0",
			Environment:    "development",
		},
		Scraper: ScraperConfig{
			UserAgents: []string{
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/604.1",
				"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
			},
			ProxyURLs:        []string{},
			MinDelay:         3 * time.Second,
			MaxDelay:         10 * time.Second,
			Timeout:          30 * time.Second,
			MaxResponseSize:  10 << 20,
			RespectRobotsTxt: true,
			MaxRetries:       3,
			BlockStatusCodes: []int{403, 429},
			BlockPatterns:    []string{"captcha", "blocked", "access denied"},
		},
		Ai: AiConfig{
			GeminiModel:   "gemini-2.0-flash",
			GeminiTimeout: 10 * time.Second,
			CacheTTL:      24 * time.Hour,
			BatchSize:     10,
		},
		Auth: AuthConfig{
			JWTAlgorithms: []string{"HS256"},
			BearerPrefix:  "Bearer",
			SkipPaths:     []string{"/healthz", "/ready", "/metrics"},
		},
		HTTP: HTTPConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
				Burst:             20,
			},
			CORS: CORSConfig{
				AllowedOrigins:    []string{"*"},
				AllowedMethods:    []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
				AllowedHeaders:    []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
		},
		Logging: LoggingConfig{
			Level:      "debug",
			Format:     "console",
			TimeFormat: "RFC3339",
		},
	}
}
