// Package config provides Viper-based configuration loading for all services.
// It supports layered YAML files, environment variable overrides, and secret injection.
package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration type. Services embed this or a subset of
// its fields and call Load to populate it from application.yaml/config.yaml and environment.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	MongoDB   MongoDBConfig   `mapstructure:"mongodb"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Scraper   ScraperConfig   `mapstructure:"scraper"`
	Trending  TrendingConfig  `mapstructure:"trending"`
	Ai        AiConfig        `mapstructure:"ai"`
	Auth      AuthConfig      `mapstructure:"auth"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Discord   DiscordConfig   `mapstructure:"discord"`
	Telegram  TelegramConfig  `mapstructure:"telegram"`
	WhatsApp  WhatsAppConfig  `mapstructure:"whatsapp"`
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	Bot       BotConfig       `mapstructure:"bot"`
	Modules   ModulesConfig   `mapstructure:"modules"`
	Compose   ComposeConfig   `mapstructure:"compose"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Tenant    TenantConfig    `mapstructure:"tenant"`
	R2        R2Config        `mapstructure:"r2"`
	GDrive    GDriveConfig    `mapstructure:"gdrive"`
	Analytics AnalyticsConfig `mapstructure:"analytics"`
	Lifecycle LifecycleConfig `mapstructure:"lifecycle"`
}

// DiscoveryConfig holds service discovery settings. Maps to config.yaml "discovery:".
type DiscoveryConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Backend string        `mapstructure:"backend"` // "static" | "consul" | "dns"
	Consul  ConsulDiscCfg `mapstructure:"consul"`
	DNS     DNSDiscCfg    `mapstructure:"dns"`
	Static  StaticDiscCfg `mapstructure:"static"`
	TTL     time.Duration `mapstructure:"ttl"` // default TTL for heartbeats
}

// ConsulDiscCfg holds Consul agent settings for service discovery.
type ConsulDiscCfg struct {
	Addr           string        `mapstructure:"addr"`
	Datacenter     string        `mapstructure:"datacenter"`
	Token          string        `mapstructure:"token"`
	HealthInterval time.Duration `mapstructure:"health_check_interval"`
}

// DNSDiscCfg holds DNS-based service discovery settings.
type DNSDiscCfg struct {
	Domain string `mapstructure:"domain"`
}

// StaticDiscCfg holds static service endpoint configurations for local dev.
type StaticDiscCfg struct {
	Services map[string][]StaticDiscEntry `mapstructure:"services"`
}

// StaticDiscEntry represents a single static service endpoint.
type StaticDiscEntry struct {
	Address  string            `mapstructure:"address"`
	Tags     []string          `mapstructure:"tags"`
	Metadata map[string]string `mapstructure:"metadata"`
	Version  string            `mapstructure:"version"`
}

// TenantConfig holds multi-tenant isolation settings. Maps to config.yaml "tenant:".
type TenantConfig struct {
	Enabled     bool                 `mapstructure:"enabled"`
	Isolation   string               `mapstructure:"isolation"` // "collection" or "field"
	DefaultID   string               `mapstructure:"default_id"`
	Definitions map[string]TenantDef `mapstructure:"definitions"`
}

// R2Config holds Cloudflare R2 (S3-compatible object storage) settings.
// Maps to config.yaml "r2:".
type R2Config struct {
	BucketName   string `mapstructure:"bucket_name"`   // R2 bucket name
	Endpoint     string `mapstructure:"endpoint"`      // https://<accountid>.r2.cloudflarestorage.com
	AccessKeyID  string `mapstructure:"access_key_id"` // R2 API token ID
	SecretKey    string `mapstructure:"secret_key"`    // R2 API token secret
	PublicDomain string `mapstructure:"public_domain"` // CDN public base URL, e.g. https://pub.example.com
	Region       string `mapstructure:"region"`        // R2 region, use "auto" by convention
}

// AnalyticsConfig holds Firebase Analytics sync settings.
type AnalyticsConfig struct {
	FirebaseAPIKey string `mapstructure:"firebase_api_key"` // API key for Firebase → erg-go sync
}

// LifecycleConfig controls whether noncritical one-off work runs during default startup.
type LifecycleConfig struct {
	AuthBootstrapAdminOnStartup bool `mapstructure:"auth_bootstrap_admin_on_startup"`
	ProfileBackfillOnStartup    bool `mapstructure:"profile_backfill_on_startup"`
	OperationSeedOnStartup      bool `mapstructure:"operation_seed_on_startup"`
	TrendingRefreshOnStartup    bool `mapstructure:"trending_refresh_on_startup"`
	LMSSeedOnStartup            bool `mapstructure:"lms_seed_on_startup"`
}

// GDriveConfig holds Google Drive storage settings.
type GDriveConfig struct {
	CredentialJSON string `mapstructure:"credential_json"` // Service account JSON (inline or path)
	FolderID       string `mapstructure:"folder_id"`       // Root folder ID for disclosure docs
}

// TenantDef defines a single tenant's configuration.
type TenantDef struct {
	DisplayName string            `mapstructure:"display_name"`
	Enabled     bool              `mapstructure:"enabled"`
	Metadata    map[string]string `mapstructure:"metadata"`
}

// ModulesConfig holds the plugin/module composition configuration.
// Phase 4 (task3.md): allows consumers to select which modules to load.
type ModulesConfig struct {
	// Enabled lists which modules to register. Valid values: "bot", "crawler",
	// "notification", "trending". If empty, all 4 modules are loaded (legacy behaviour).
	Enabled []string `mapstructure:"enabled"`
}

// ComposeConfig holds config-driven composition settings.
// Maps to config.yaml "compose:".
type ComposeConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	DeployManifestPath string `mapstructure:"deploy_manifest_path"`
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
	Host             string        `mapstructure:"host"`
	Port             int           `mapstructure:"port"`
	User             string        `mapstructure:"user"`
	Password         string        `mapstructure:"password"`
	Name             string        `mapstructure:"name"`
	MaxOpenConns     int           `mapstructure:"max_open_conns"`
	MaxIdleConns     int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime  time.Duration `mapstructure:"conn_max_idle_time"`
	AutoMigrate      bool          `mapstructure:"auto_migrate"`
	RunBackfills     bool          `mapstructure:"run_backfills"`
	MigrationTimeout time.Duration `mapstructure:"migration_timeout"`
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
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	TLS          bool          `mapstructure:"tls"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
}

// QueueConfig holds Asynq queue settings.
type QueueConfig struct {
	RedisUsername string        `mapstructure:"redis_username"`
	RedisHost     string        `mapstructure:"redis_host"`
	RedisPort     int           `mapstructure:"redis_port"`
	RedisPassword string        `mapstructure:"redis_password"`
	RedisDB       int           `mapstructure:"redis_db"`
	RedisTLS      bool          `mapstructure:"redis_tls"`
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

// TrendingConfig holds trending aggregation settings.
type TrendingConfig struct {
	RefreshCron string        `mapstructure:"refresh_cron"`
	CacheTTL    time.Duration `mapstructure:"cache_ttl"`
	FeedLimit   int           `mapstructure:"feed_limit"`
	TopicsLimit int           `mapstructure:"topics_limit"`
	NewsLimit   int           `mapstructure:"news_limit"`
	MinHotScore float64       `mapstructure:"min_hot_score"`
}

// AiConfig holds AI service settings.
type AiConfig struct {
	Provider                       string        `mapstructure:"provider"`
	APIKeyEncryptionSecret         string        `mapstructure:"api_key_encryption_secret"`
	GeminiAPIKey                   string        `mapstructure:"gemini_api_key"`
	GeminiModel                    string        `mapstructure:"gemini_model"`
	GeminiTimeout                  time.Duration `mapstructure:"gemini_timeout"`
	GroqAPIKey                     string        `mapstructure:"groq_api_key"`
	GroqModel                      string        `mapstructure:"groq_model"`
	GroqBaseURL                    string        `mapstructure:"groq_base_url"`
	GroqTimeout                    time.Duration `mapstructure:"groq_timeout"`
	GroqMaxCompletionTokens        int           `mapstructure:"groq_max_completion_tokens"`
	GroqTemperature                float64       `mapstructure:"groq_temperature"`
	GroqReasoningEffort            string        `mapstructure:"groq_reasoning_effort"`
	HuggingFaceImageAPIKey         string        `mapstructure:"huggingface_image_api_key"`
	HuggingFaceImageProvider       string        `mapstructure:"huggingface_image_provider"`
	HuggingFaceImageModel          string        `mapstructure:"huggingface_image_model"`
	HuggingFaceImageBaseURL        string        `mapstructure:"huggingface_image_base_url"`
	HuggingFaceImageTimeout        time.Duration `mapstructure:"huggingface_image_timeout"`
	HuggingFaceImageWidth          int           `mapstructure:"huggingface_image_width"`
	HuggingFaceImageHeight         int           `mapstructure:"huggingface_image_height"`
	HuggingFaceImageSteps          int           `mapstructure:"huggingface_image_steps"`
	HuggingFaceImageGuidance       float64       `mapstructure:"huggingface_image_guidance"`
	HuggingFaceImageNegativePrompt string        `mapstructure:"huggingface_image_negative_prompt"`
	CacheTTL                       time.Duration `mapstructure:"cache_ttl"`
	BatchSize                      int           `mapstructure:"batch_size"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTRefreshSecret   string        `mapstructure:"jwt_refresh_secret"`
	JWTPublicKey       string        `mapstructure:"jwt_public_key"`
	JWTIssuer          string        `mapstructure:"jwt_issuer"`
	GoogleBridgeSecret string        `mapstructure:"google_bridge_secret"`
	GoogleClientID     string        `mapstructure:"google_client_id"`
	GoogleClientIDs    []string      `mapstructure:"google_client_ids"`
	JWTAlgorithms      []string      `mapstructure:"jwt_algorithms"`
	BearerPrefix       string        `mapstructure:"bearer_prefix"`
	SkipPaths          []string      `mapstructure:"skip_paths"`
	AccessTokenTTL     time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL    time.Duration `mapstructure:"refresh_token_ttl"`
	Argon2Memory       uint32        `mapstructure:"argon2_memory"`
	Argon2Iterations   uint32        `mapstructure:"argon2_iterations"`
	MaxFailedLogin     int           `mapstructure:"max_failed_login"`
	FailedLoginWindow  time.Duration `mapstructure:"failed_login_window"`
	BlockDuration      time.Duration `mapstructure:"block_duration"`
	GeoBlockEnabled    bool          `mapstructure:"geo_block_enabled"`
	BlockUnknownGeo    bool          `mapstructure:"block_unknown_geo"`
	AllowedContinents  []string      `mapstructure:"allowed_continents"`
	// BootstrapAdmin controls whether a default super-admin account is created on startup.
	AdminEmail    string           `mapstructure:"admin_email"`
	AdminPassword string           `mapstructure:"admin_password"`
	Cookie        AuthCookieConfig `mapstructure:"cookie"`
}

// AuthCookieConfig controls browser session cookies for direct API mode.
type AuthCookieConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	Domain           string `mapstructure:"domain"`
	Secure           bool   `mapstructure:"secure"`
	SameSite         string `mapstructure:"same_site"`
	AccessTokenName  string `mapstructure:"access_token_name"`
	RefreshTokenName string `mapstructure:"refresh_token_name"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Host              string          `mapstructure:"host"`
	Port              int             `mapstructure:"port"`
	ReadTimeout       time.Duration   `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration   `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration   `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration   `mapstructure:"shutdown_timeout"`
	TrustedProxyCIDRs []string        `mapstructure:"trusted_proxy_cidrs"`
	RateLimit         RateLimitConfig `mapstructure:"rate_limit"`
	CORS              CORSConfig      `mapstructure:"cors"`
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
	MaxAge           int      `mapstructure:"max_age"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	TimeFormat string `mapstructure:"time_format"`
}

// DiscordConfig holds Discord bot and webhook settings.
type DiscordConfig struct {
	Token         string `mapstructure:"token"`          // Bot token (Bot <token>)
	PublicKey     string `mapstructure:"public_key"`     // Ed25519 public key for webhook verification
	WebhookSecret string `mapstructure:"webhook_secret"` // HMAC secret for X-Hub-Signature-256 verification
	WebhookURL    string `mapstructure:"webhook_url"`    // Default Discord webhook URL for notifications
	GuildID       string `mapstructure:"guild_id"`       // Default guild ID (optional)
}

// WhatsAppConfig holds WhatsApp Business API settings.
type WhatsAppConfig struct {
	PhoneID     string `mapstructure:"phone_id"`     // WhatsApp Business Phone ID
	AccessToken string `mapstructure:"access_token"` // WhatsApp Business API access token
	VerifyToken string `mapstructure:"verify_token"` // Webhook verify token
}

// SMTPConfig holds email SMTP settings.
type SMTPConfig struct {
	Host     string `mapstructure:"host"`     // SMTP server host
	Port     int    `mapstructure:"port"`     // SMTP port (default: 587)
	Username string `mapstructure:"username"` // SMTP username
	Password string `mapstructure:"password"` // SMTP password
	From     string `mapstructure:"from"`     // Sender email address
	TLS      bool   `mapstructure:"tls"`      // Use TLS (default: true)
}

// TelegramConfig holds Telegram bot settings.
type TelegramConfig struct {
	BotToken      string `mapstructure:"bot_token"`      // Bot API token from @BotFather
	WebhookSecret string `mapstructure:"webhook_secret"` // HMAC secret for webhook verification
	WebhookURL    string `mapstructure:"webhook_url"`    // Public webhook URL for Telegram
}

// BotConfig holds bot-specific settings.
type BotConfig struct {
	CommandPrefix       string   `mapstructure:"command_prefix"`       // Default: "/"
	AdminIDs            []string `mapstructure:"admin_ids"`            // User IDs with admin access
	AllowedGuilds       []string `mapstructure:"allowed_guilds"`       // Discord guilds allowed to use bot
	MaxConversations    int      `mapstructure:"max_conversations"`    // Max concurrent conversations
	WizardTTLMinutes    int      `mapstructure:"wizard_ttl_minutes"`   // Wizard session TTL (default: 5)
	RateLimitPerUser    int      `mapstructure:"rate_limit_per_user"`  // Max commands per user per minute
	EnableLinkCommand   bool     `mapstructure:"enable_link_command"`  // Enable account linking command
	NotificationChannel string   `mapstructure:"notification_channel"` // Discord channel for system notifications
}

var (
	globalViper = viper.New()
	globalCfg   *Config
	globalCfgMu sync.RWMutex
)

const secretPrefix = "SECRET_"

var legacyEnvKeyAliases = map[string]string{ // #nosec G101 -- keys are environment variable names and config paths, not credential values.
	"APP_ENV":                      "app.env",
	"NODE_ENV":                     "app.env",
	"DB_HOST":                      "database.host",
	"DB_PORT":                      "database.port",
	"DB_USER":                      "database.user",
	"DB_PASSWORD":                  "database.password",
	"DB_PASS":                      "database.password",
	"DB_NAME":                      "database.name",
	"JWT_SECRET":                   "auth.jwt_secret",
	"JWT_ACCESS_SECRET":            "auth.jwt_secret",
	"JWT_PUBLIC_KEY":               "auth.jwt_public_key",
	"JWT_ACCESS_EXPIRATION_TIME":   "auth.access_token_ttl",
	"JWT_REFRESH_EXPIRATION_TIME":  "auth.refresh_token_ttl",
	"GOOGLE_CLIENT_ID":             "auth.google_client_id",
	"GOOGLE_OAUTH_CLIENT_ID":       "auth.google_client_id",
	"MONGO_URI":                    "mongodb.uri",
	"MONGO_URL":                    "mongodb.uri",
	"MONGO_DB":                     "mongodb.database",
	"MONGO_DB_NAME":                "mongodb.database",
	"REDIS_HOST":                   "redis.host",
	"REDIS_PORT":                   "redis.port",
	"REDIS_PASS":                   "redis.password",
	"REDIS_PASSWORD":               "redis.password",
	"MAIL_HOST":                    "smtp.host",
	"MAIL_PORT":                    "smtp.port",
	"MAIL_SECURE":                  "smtp.tls",
	"MAIL_USER":                    "smtp.username",
	"MAIL_PASSWORD":                "smtp.password",
	"MAIL_FROM":                    "smtp.from",
	"R2_ENDPOINT":                  "r2.endpoint",
	"R2_ACCESS_KEY_ID":             "r2.access_key_id",
	"R2_SECRET_ACCESS_KEY":         "r2.secret_key",
	"R2_BUCKET_NAME":               "r2.bucket_name",
	"R2_PUBLIC_DOMAIN":             "r2.public_domain",
	"R2_REGION":                    "r2.region",
	"R2__BUCKET":                   "r2.bucket_name",
	"R2__SECRET_ACCESS_KEY":        "r2.secret_key",
	"AI_API_KEY_ENCRYPTION_SECRET": "ai.api_key_encryption_secret",
	"GROQ_API_KEY":                 "ai.groq_api_key",
	"GROQ_MODEL":                   "ai.groq_model",
	"GROQ_BASE_URL":                "ai.groq_base_url",
	"GEMINI_MODEL":                 "ai.gemini_model",
	"HF_TOKEN":                     "ai.huggingface_image_api_key",
	"HUGGINGFACE_TOKEN":            "ai.huggingface_image_api_key",
	"HUGGINGFACE_API_KEY":          "ai.huggingface_image_api_key",
	"HUGGINGFACE_IMAGE_PROVIDER":   "ai.huggingface_image_provider",
	"HUGGINGFACE_IMAGE_MODEL":      "ai.huggingface_image_model",
	"GDRIVE_CREDENTIAL_JSON":       "gdrive.credential_json",
	"GDRIVE_FOLDER_ID":             "gdrive.folder_id",
}

// Loader encapsulates Viper configuration options.
type Loader struct {
	configPaths []string
	envPrefix   string
	fileNames   []string
	profile     string
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

// WithFileName sets the config file name without extension.
func WithFileName(name string) LoaderOption {
	return func(l *Loader) {
		if strings.TrimSpace(name) == "" {
			return
		}
		l.fileNames = []string{name}
	}
}

// WithFileNames sets multiple config file names without extension.
// Files are merged in order, so later names override earlier names.
func WithFileNames(names ...string) LoaderOption {
	return func(l *Loader) {
		l.fileNames = l.fileNames[:0]
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			l.fileNames = append(l.fileNames, name)
		}
	}
}

// WithProfile sets the active config profile, e.g. "development" or "production".
// When empty, the loader discovers it from ERG_PROFILE, APP_PROFILE, APP__ENV,
// APP_ENV, NODE_ENV, then from app.env in the base YAML files.
func WithProfile(profile string) LoaderOption {
	return func(l *Loader) {
		l.profile = strings.TrimSpace(profile)
	}
}

// NewLoader creates a new config loader with sensible defaults.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		configPaths: []string{"."},
		envPrefix:   "",
		fileNames:   []string{"application", "config"},
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Load reads configuration from layered YAML files and environment variables into out.
//
// Merge order, from lowest to highest precedence:
//   - application.yaml, then config.yaml
//   - application.<profile>.yaml, then config.<profile>.yaml
//   - application.local.yaml, then config.local.yaml
//   - application.<profile>.local.yaml, then config.<profile>.local.yaml
//   - .env values
//   - process environment variables
func (l *Loader) Load(out interface{}) error {
	v := viper.New()

	for _, p := range l.configPaths {
		v.AddConfigPath(p)
	}
	v.SetConfigType("yaml")

	v.SetEnvPrefix(l.envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	dotEnv := loadDotEnv(l.configPaths)
	processEnv := envMapFromEnviron(os.Environ())

	if err := l.mergeBaseConfig(v); err != nil {
		return err
	}
	profile := l.resolveProfile(v, dotEnv, processEnv)
	if err := l.mergeProfileConfig(v, profile); err != nil {
		return err
	}
	if err := l.mergeLocalConfig(v, profile); err != nil {
		return err
	}

	applyEnvOverrides(v, dotEnv)
	applyEnvOverrides(v, processEnv)

	if err := v.Unmarshal(out); err != nil {
		return fmt.Errorf("config: unmarshal into struct: %w", err)
	}

	cfg, ok := out.(*Config)
	if !ok {
		return fmt.Errorf("config: expected *Config, got %T", out)
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}

	globalCfgMu.Lock()
	globalCfg = cfg
	globalViper = v // sync so GetString/GetInt/GetBool helpers read correct values
	globalCfgMu.Unlock()

	return nil
}

func (l *Loader) normalizedFileNames() []string {
	if len(l.fileNames) == 0 {
		return []string{"application", "config"}
	}
	names := make([]string, 0, len(l.fileNames))
	seen := make(map[string]struct{}, len(l.fileNames))
	for _, name := range l.fileNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return []string{"application", "config"}
	}
	return names
}

func (l *Loader) mergeBaseConfig(v *viper.Viper) error {
	for _, name := range l.normalizedFileNames() {
		if err := l.mergeNamedConfig(v, name); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) mergeProfileConfig(v *viper.Viper, profile string) error {
	if profile == "" {
		return nil
	}
	for _, name := range l.normalizedFileNames() {
		if err := l.mergeNamedConfig(v, name+"."+profile); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) mergeLocalConfig(v *viper.Viper, profile string) error {
	for _, name := range l.normalizedFileNames() {
		if err := l.mergeNamedConfig(v, name+".local"); err != nil {
			return err
		}
	}
	if profile == "" {
		return nil
	}
	for _, name := range l.normalizedFileNames() {
		if err := l.mergeNamedConfig(v, name+"."+profile+".local"); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) mergeNamedConfig(v *viper.Viper, name string) error {
	v.SetConfigName(name)
	v.SetConfigType("yaml")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("config: read %s.yaml: %w", name, err)
	}
	return nil
}

func (l *Loader) resolveProfile(v *viper.Viper, dotEnv, processEnv map[string]string) string {
	if l.profile != "" {
		return normalizeProfile(l.profile)
	}
	for _, entries := range []map[string]string{processEnv, dotEnv} {
		for _, key := range []string{"ERG_PROFILE", "APP_PROFILE", "APP__ENV", "APP_ENV", "NODE_ENV"} {
			if val := normalizeProfile(entries[key]); val != "" {
				return val
			}
		}
	}
	return normalizeProfile(v.GetString("app.env"))
}

func normalizeProfile(profile string) string {
	profile = strings.TrimSpace(strings.ToLower(profile))
	profile = strings.ReplaceAll(profile, "_", "-")
	if profile == "local" {
		return ""
	}
	return profile
}

func applyEnvOverrides(v *viper.Viper, entries map[string]string) {
	for name, value := range entries {
		keyName := name
		if strings.HasPrefix(name, secretPrefix) {
			keyName = strings.TrimPrefix(name, secretPrefix)
		}

		if !strings.Contains(keyName, "__") {
			if _, ok := legacyEnvKeyAliases[strings.ToUpper(keyName)]; !ok && !strings.HasPrefix(name, secretPrefix) {
				continue
			}
		}

		key := normalizeEnvKey(keyName)
		if key == "" {
			continue
		}
		v.Set(key, normalizeEnvValue(key, value))
	}
}

func envMapFromEnviron(environ []string) map[string]string {
	entries := make(map[string]string, len(environ))
	for _, e := range environ {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		entries[parts[0]] = parts[1]
	}
	return entries
}

func loadDotEnv(paths []string) map[string]string {
	entries := make(map[string]string)
	seen := make(map[string]struct{}, len(paths))

	for _, dir := range paths {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, ".env")
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		file, err := os.Open(path) // #nosec G304 -- path is a configured search directory joined with the fixed ".env" filename.
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			key, value, ok := parseDotEnvLine(scanner.Text())
			if !ok {
				continue
			}
			entries[key] = value
		}

		_ = file.Close()
	}

	return entries
}

func parseDotEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}

	idx := strings.IndexRune(line, '=')
	if idx <= 0 {
		return "", "", false
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}

func normalizeEnvKey(name string) string {
	if name == "" {
		return ""
	}

	if alias, ok := legacyEnvKeyAliases[strings.ToUpper(name)]; ok {
		return alias
	}

	if strings.Contains(name, "__") {
		parts := strings.Split(name, "__")
		normalized := make([]string, 0, len(parts))
		for i, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			part = strings.ToLower(part)
			if i == 0 {
				switch part {
				case "db":
					part = "database"
				case "mongo":
					part = "mongodb"
				}
			}
			normalized = append(normalized, part)
		}
		return strings.Join(normalized, ".")
	}

	return strings.ToLower(strings.ReplaceAll(name, "_", "."))
}

func normalizeEnvValue(key, value string) string {
	if !isDurationLikeKey(key) {
		return value
	}

	if len(value) < 2 || !strings.HasSuffix(value, "d") {
		return value
	}

	days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
	if err != nil {
		return value
	}

	return fmt.Sprintf("%dh", days*24)
}

func isDurationLikeKey(key string) bool {
	switch {
	case strings.HasSuffix(key, ".read_timeout"),
		strings.HasSuffix(key, ".write_timeout"),
		strings.HasSuffix(key, ".idle_timeout"),
		strings.HasSuffix(key, ".shutdown_timeout"),
		strings.HasSuffix(key, ".dial_timeout"),
		strings.HasSuffix(key, ".connect_timeout"),
		strings.HasSuffix(key, ".socket_timeout"),
		strings.HasSuffix(key, ".server_selection_timeout"),
		strings.HasSuffix(key, ".retry_delay"),
		strings.HasSuffix(key, ".cache_ttl"),
		strings.HasSuffix(key, ".access_token_ttl"),
		strings.HasSuffix(key, ".refresh_token_ttl"),
		strings.HasSuffix(key, ".conn_max_lifetime"),
		strings.HasSuffix(key, ".conn_max_idle_time"),
		strings.HasSuffix(key, ".block_duration"),
		strings.HasSuffix(key, ".failed_login_window"),
		strings.HasSuffix(key, ".min_delay"),
		strings.HasSuffix(key, ".max_delay"),
		strings.HasSuffix(key, ".timeout"),
		strings.HasSuffix(key, ".health_check_interval"),
		strings.HasSuffix(key, ".ttl"):
		return true
	default:
		return false
	}
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config: nil config")
	}
	if err := validateTrustedProxyCIDRs(cfg.HTTP.TrustedProxyCIDRs); err != nil {
		return err
	}
	if strings.EqualFold(cfg.App.Env, "production") {
		for _, origin := range cfg.HTTP.CORS.AllowedOrigins {
			if origin == "*" {
				return fmt.Errorf("config: http.cors.allowed_origins cannot contain '*' in production")
			}
		}
		if cfg.Database.AutoMigrate {
			return fmt.Errorf("config: database.auto_migrate must be false in production; run cmd/db-migrate instead")
		}
		if cfg.Database.RunBackfills {
			return fmt.Errorf("config: database.run_backfills must be false in production; run cmd/db-migrate -backfill instead")
		}
		if cfg.Auth.JWTSecret == "" && cfg.Auth.JWTPublicKey == "" {
			return fmt.Errorf("config: auth.jwt_secret or auth.jwt_public_key is required in production")
		}
		if cfg.Auth.JWTSecret != "" {
			if err := validateProductionSecret("auth.jwt_secret", cfg.Auth.JWTSecret, 32); err != nil {
				return err
			}
			if err := validateProductionSecret("auth.jwt_refresh_secret", cfg.Auth.JWTRefreshSecret, 32); err != nil {
				return err
			}
		}
		if cfg.Auth.Cookie.Enabled && !cfg.Auth.Cookie.Secure {
			return fmt.Errorf("config: auth.cookie.secure must be true in production")
		}
		if err := validateProductionSecret("database.password", cfg.Database.Password, 8); err != nil {
			return err
		}
		if cfg.Redis.Host != "" && cfg.Redis.Host != "localhost" && cfg.Redis.Host != "127.0.0.1" {
			if err := validateProductionSecret("redis.password", cfg.Redis.Password, 8); err != nil {
				return err
			}
		}
		if cfg.Queue.RedisHost != "" && cfg.Queue.RedisHost != "localhost" && cfg.Queue.RedisHost != "127.0.0.1" {
			if err := validateProductionSecret("queue.redis_password", cfg.Queue.RedisPassword, 8); err != nil {
				return err
			}
		}
		if cfg.SMTP.Host != "" {
			if err := validateProductionSecret("smtp.password", cfg.SMTP.Password, 8); err != nil {
				return err
			}
		}
		if cfg.R2.Endpoint != "" || cfg.R2.BucketName != "" {
			if err := validateProductionSecret("r2.access_key_id", cfg.R2.AccessKeyID, 8); err != nil {
				return err
			}
			if err := validateProductionSecret("r2.secret_key", cfg.R2.SecretKey, 16); err != nil {
				return err
			}
		}
		if err := validateProductionSecret("ai.api_key_encryption_secret", cfg.Ai.APIKeyEncryptionSecret, 32); err != nil {
			return err
		}
	}
	return nil
}

func validateTrustedProxyCIDRs(cidrs []string) error {
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(cidr); err == nil {
			continue
		}
		if ip := net.ParseIP(cidr); ip != nil {
			continue
		}
		return fmt.Errorf("config: http.trusted_proxy_cidrs contains invalid CIDR or IP %q", cidr)
	}
	return nil
}

func validateProductionSecret(name, value string, minLen int) error {
	value = strings.TrimSpace(value)
	if len(value) < minLen {
		return fmt.Errorf("config: %s must be at least %d characters in production", name, minLen)
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"replace-with", "change-in-production", "changeme", "example", "dummy"} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("config: %s contains placeholder value in production", name)
		}
	}
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
			Host:             "localhost",
			Port:             5432,
			User:             "postgres",
			Password:         "postgres",
			Name:             "erg",
			MaxOpenConns:     25,
			MaxIdleConns:     5,
			ConnMaxLifetime:  30 * time.Minute,
			ConnMaxIdleTime:  5 * time.Minute,
			AutoMigrate:      false,
			RunBackfills:     false,
			MigrationTimeout: 2 * time.Minute,
		},
		MongoDB: MongoDBConfig{
			URI:                    "mongodb://localhost:27017",
			Database:               "erg",
			MaxPoolSize:            100,
			MinPoolSize:            10,
			ServerSelectionTimeout: 10 * time.Second,
			ConnectTimeout:         10 * time.Second,
			ReadPreference:         "secondaryPreferred",
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Username:     "",
			DB:           0,
			TLS:          false,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
		},
		Queue: QueueConfig{
			RedisUsername: "",
			RedisTLS:      false,
			Concurrency:   10,
			RetryDelay:    10 * time.Second,
			MaxRetry:      3,
			RetryBackoff:  true,
			QueueName:     "default",
			DLQQueueName:  "erg-dlq",
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
		Trending: TrendingConfig{
			RefreshCron: "*/30 * * * *",
			CacheTTL:    25 * time.Minute,
			FeedLimit:   100,
			TopicsLimit: 20,
			NewsLimit:   20,
			MinHotScore: 75,
		},
		Ai: AiConfig{
			Provider:                       "gemini",
			GeminiModel:                    "gemini-2.0-flash",
			GeminiTimeout:                  10 * time.Second,
			GroqModel:                      "openai/gpt-oss-120b",
			GroqBaseURL:                    "https://api.groq.com/openai/v1",
			GroqTimeout:                    30 * time.Second,
			GroqMaxCompletionTokens:        4096,
			GroqTemperature:                1,
			GroqReasoningEffort:            "medium",
			HuggingFaceImageProvider:       "fal-ai",
			HuggingFaceImageModel:          "black-forest-labs/FLUX.1-Krea-dev",
			HuggingFaceImageBaseURL:        "https://router.huggingface.co",
			HuggingFaceImageTimeout:        90 * time.Second,
			HuggingFaceImageWidth:          1024,
			HuggingFaceImageHeight:         576,
			HuggingFaceImageSteps:          28,
			HuggingFaceImageGuidance:       7,
			HuggingFaceImageNegativePrompt: "text, watermark, logo, blurry, distorted, low quality, extra fingers",
			CacheTTL:                       24 * time.Hour,
			BatchSize:                      10,
		},
		Auth: AuthConfig{
			JWTAlgorithms:     []string{"HS256"},
			BearerPrefix:      "Bearer",
			SkipPaths:         []string{"/healthz", "/ready", "/metrics"},
			AccessTokenTTL:    15 * time.Minute,
			RefreshTokenTTL:   7 * 24 * time.Hour,
			Argon2Memory:      32768,
			Argon2Iterations:  2,
			MaxFailedLogin:    5,
			FailedLoginWindow: 15 * time.Minute,
			BlockDuration:     15 * time.Minute,
			GeoBlockEnabled:   true,
			BlockUnknownGeo:   false,
			AllowedContinents: []string{"AS"},
			Cookie: AuthCookieConfig{
				Enabled:          true,
				Secure:           false,
				SameSite:         "lax",
				AccessTokenName:  "erg_access_token",
				RefreshTokenName: "erg_refresh_token",
			},
		},
		HTTP: HTTPConfig{
			Host:              "0.0.0.0",
			Port:              8080,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   30 * time.Second,
			TrustedProxyCIDRs: []string{},
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
				Burst:             20,
			},
			CORS: CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
				AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-CSRF-Token"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
		},
		Logging: LoggingConfig{
			Level:      "debug",
			Format:     "console",
			TimeFormat: "RFC3339",
		},
		Discord: DiscordConfig{
			Token: "",
		},
		Telegram: TelegramConfig{
			BotToken: "",
		},
		WhatsApp: WhatsAppConfig{},
		SMTP: SMTPConfig{
			Port: 587,
			TLS:  true,
		},
		Bot: BotConfig{
			CommandPrefix:     "/",
			AdminIDs:          []string{},
			AllowedGuilds:     []string{},
			MaxConversations:  10000,
			WizardTTLMinutes:  5,
			RateLimitPerUser:  20,
			EnableLinkCommand: true,
		},
		Modules: ModulesConfig{
			// Enabled is empty by default → all 4 modules loaded (legacy behaviour).
			// Set in config.yaml to e.g. ["crawler", "notification"] for selective loading.
			Enabled: []string{},
		},
		R2: R2Config{
			Region: "auto",
		},
		Lifecycle: LifecycleConfig{},
	}
}
