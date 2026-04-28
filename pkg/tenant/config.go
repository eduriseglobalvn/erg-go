package tenant

import (
	"encoding/json"
	"fmt"
	"time"
)

// IsolationMode defines how tenant data is isolated at the storage layer.
type IsolationMode string

const (
	// IsolationCollection uses per-tenant collection names (e.g. "acme_crawl_histories").
	// Best for workloads requiring strong tenant boundaries.
	IsolationCollection IsolationMode = "collection"
	// IsolationField stores all tenants in shared collections and filters by tenant_id field.
	// Simpler operations but lower isolation guarantees.
	IsolationField IsolationMode = "field"
)

// Quota defines per-tenant resource limits.
type Quota struct {
	CrawlPerHour        int `mapstructure:"crawl_per_hour" json:"crawl_per_hour"`
	NotificationsPerDay int `mapstructure:"notifications_per_day" json:"notifications_per_day"`
	CrawlConcurrent     int `mapstructure:"crawl_concurrent" json:"crawl_concurrent"`
	StorageBytesMax     int `mapstructure:"storage_bytes_max" json:"storage_bytes_max"`
}

// TenantDef defines a single tenant's configuration.
type TenantDef struct {
	DisplayName string            `mapstructure:"display_name" json:"display_name"`
	Enabled     bool              `mapstructure:"enabled" json:"enabled"`
	Quota       Quota             `mapstructure:"quota" json:"quota"`
	Metadata    map[string]string `mapstructure:"metadata" json:"metadata,omitempty"`
}

// TenantConfig holds the multi-tenancy section of the application config.
type TenantConfig struct {
	Enabled     bool                 `mapstructure:"enabled" json:"enabled"`
	Isolation   IsolationMode        `mapstructure:"isolation" json:"isolation"`
	DefaultID   string               `mapstructure:"default" json:"default"`
	Definitions map[string]TenantDef `mapstructure:"definitions" json:"definitions,omitempty"`
}

// DefaultTenantID returns the configured default tenant, falling back to "default".
func (c *TenantConfig) DefaultTenantID() string {
	if c.DefaultID != "" {
		return c.DefaultID
	}
	return "default"
}

// Lookup returns the TenantDef for a given ID and a boolean indicating whether it exists.
func (c *TenantConfig) Lookup(id string) (TenantDef, bool) {
	if c.Definitions == nil {
		return TenantDef{}, false
	}
	def, ok := c.Definitions[id]
	return def, ok
}

// IsEnabled reports whether multi-tenancy is active.
func (c *TenantConfig) IsEnabled() bool {
	return c.Enabled
}

// ConfigLoader adapts the global config.TenantConfig into the pkg/tenant type system.
type ConfigLoader struct{}

// NewConfigLoader returns a new ConfigLoader.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

// LoadFromConfig extracts TenantConfig from a raw map loaded from config.yaml.
// The expected map structure matches the TenantConfig struct fields.
// If m is nil or empty, returns a disabled-by-default TenantConfig.
func (l *ConfigLoader) LoadFromConfig(m map[string]interface{}) *TenantConfig {
	if m == nil {
		return &TenantConfig{Enabled: false, DefaultID: "default"}
	}
	tc := &TenantConfig{DefaultID: "default"}

	if v, ok := m["enabled"].(bool); ok {
		tc.Enabled = v
	}
	if v, ok := m["isolation"].(string); ok {
		tc.Isolation = IsolationMode(v)
	}
	if v, ok := m["default"].(string); ok {
		tc.DefaultID = v
	}
	_ = tc // silence
	return tc
}

// CacheConfig holds cache TTL and size settings per tenant.
type CacheConfig struct {
	TTL               time.Duration `mapstructure:"ttl" json:"ttl"`
	MaxEntries        int           `mapstructure:"max_entries" json:"max_entries"`
	EnableCompression bool          `mapstructure:"enable_compression" json:"enable_compression"`
}

// QueueConfig holds queue settings per tenant.
type QueueConfig struct {
	Concurrency    int  `mapstructure:"concurrency" json:"concurrency"`
	DedicatedQueue bool `mapstructure:"dedicated_queue" json:"dedicated_queue"`
}

// MergeStrategy returns the effective TenantDef for a given ID, applying
// sensible defaults when the tenant is not explicitly defined.
func (c *TenantConfig) MergeStrategy(id string) TenantDef {
	if def, ok := c.Lookup(id); ok {
		return def
	}
	// Return a sensible default for unknown tenants.
	return TenantDef{
		DisplayName: id,
		Enabled:     true,
		Quota: Quota{
			CrawlPerHour:        500,
			NotificationsPerDay: 2000,
			CrawlConcurrent:     3,
		},
	}
}

// Validate checks that the tenant configuration is internally consistent.
func (c *TenantConfig) Validate() error {
	if c == nil {
		return nil
	}
	if c.Isolation != IsolationCollection && c.Isolation != IsolationField && c.Isolation != "" {
		return fmt.Errorf("tenant: invalid isolation mode %q (must be %q or %q)",
			c.Isolation, IsolationCollection, IsolationField)
	}
	for id, def := range c.Definitions {
		if id == "" {
			return fmt.Errorf("tenant: empty tenant ID in definitions")
		}
		if def.Quota.CrawlPerHour < 0 {
			return fmt.Errorf("tenant %q: crawl_per_hour must be non-negative", id)
		}
		if def.Quota.NotificationsPerDay < 0 {
			return fmt.Errorf("tenant %q: notifications_per_day must be non-negative", id)
		}
	}
	return nil
}

// TenantJSON is the JSON serialisable view of a tenant for API responses.
type TenantJSON struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	Enabled     bool              `json:"enabled"`
	Quota       Quota             `json:"quota"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ToJSON converts TenantDef to TenantJSON with the given tenant ID.
func (def TenantDef) ToJSON(id string) TenantJSON {
	return TenantJSON{
		ID:          id,
		DisplayName: def.DisplayName,
		Enabled:     def.Enabled,
		Quota:       def.Quota,
		Metadata:    def.Metadata,
	}
}

// MarshalJSON implements json.Marshaler for TenantDef.
func (def TenantDef) MarshalJSON() ([]byte, error) {
	return json.Marshal(def.ToJSON(""))
}
