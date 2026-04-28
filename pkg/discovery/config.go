package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Config holds the raw config values loaded from config.yaml.
type Config struct {
	Enabled bool          `mapstructure:"enabled"`
	Backend string        `mapstructure:"backend"` // "consul", "dns", "static"
	Consul  ConsulCfg     `mapstructure:"consul"`
	DNS     DNSCfg        `mapstructure:"dns"`
	Static  StaticCfg     `mapstructure:"static"`
	TTL     time.Duration `mapstructure:"ttl"` // default TTL for heartbeats
}

// ConsulCfg is the HashiCorp Consul backend configuration.
type ConsulCfg struct {
	Addr           string        `mapstructure:"addr"`                  // "consul.internal.erg.ninja:8500"
	Datacenter     string        `mapstructure:"datacenter"`            // "dc1"
	Token          string        `mapstructure:"token"`                 // read from env var
	HealthInterval time.Duration `mapstructure:"health_check_interval"` // default 10s
}

// DNSCfg is the DNS SRV-based backend configuration.
type DNSCfg struct {
	Domain string `mapstructure:"domain"` // "internal.erg.ninja"
}

// StaticCfg is the static in-memory catalog configuration.
type StaticCfg struct {
	Services map[string][]StaticServiceEntry `mapstructure:"services"`
}

// StaticServiceEntry is a statically defined service instance.
type StaticServiceEntry struct {
	Address  string            `mapstructure:"address"` // "localhost:8083"
	Tags     []string          `mapstructure:"tags"`
	Metadata map[string]string `mapstructure:"metadata"`
	Version  string            `mapstructure:"version"`
}

// BuildCatalog creates the appropriate Catalog from the Config.
// Token placeholders of the form "${ENV_VAR}" are resolved from the environment.
func BuildCatalog(ctx context.Context, cfg Config) (Catalog, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("discovery: disabled by config")
	}
	switch cfg.Backend {
	case "consul":
		return buildConsulCatalog(ctx, cfg)
	case "dns":
		return buildDNSCatalog(ctx, cfg)
	case "static":
		return buildStaticCatalog(ctx, cfg)
	default:
		// Default to static catalog for local development.
		return buildStaticCatalog(ctx, cfg)
	}
}

// buildConsulCatalog returns a consul-backed catalog.
// Falls back to a no-op StaticCatalog if CONSUL_ADDR is not set.
func buildConsulCatalog(_ context.Context, cfg Config) (Catalog, error) {
	addr := resolveEnv(cfg.Consul.Addr)
	if addr == "" {
		addr = os.Getenv("CONSUL_ADDR")
	}
	if addr == "" {
		// Consul not configured; fall back to static in-memory.
		return &StaticCatalog{}, nil
	}
	token := resolveEnv(cfg.Consul.Token)
	if token == "" {
		token = os.Getenv("CONSUL_TOKEN")
	}
	return &ConsulCatalog{
		addr:       addr,
		token:      token,
		datacenter: cfg.Consul.Datacenter,
	}, nil
}

// buildDNSCatalog returns a DNS-SRV-based catalog.
func buildDNSCatalog(_ context.Context, cfg Config) (Catalog, error) {
	if cfg.DNS.Domain == "" {
		cfg.DNS.Domain = "internal.erg.ninja"
	}
	return &DNSCatalog{domain: cfg.DNS.Domain}, nil
}

// buildStaticCatalog creates an in-memory catalog from the static config.
func buildStaticCatalog(_ context.Context, cfg Config) (Catalog, error) {
	m := make(map[string][]Service)
	for name, entries := range cfg.Static.Services {
		for _, e := range entries {
			m[name] = append(m[name], Service{
				ID:       fmt.Sprintf("%s-%s", name, addrToID(e.Address)),
				Name:     name,
				Version:  e.Version,
				Address:  e.Address,
				Tags:     e.Tags,
				Metadata: e.Metadata,
				TTL:      time.Time{},
			})
		}
	}
	return &StaticCatalog{svcs: m}, nil
}

// resolveEnv expands "${VAR}" and "$VAR" style placeholders from the environment.
func resolveEnv(val string) string {
	if len(val) < 2 {
		return val
	}
	if val[0] == '$' {
		if val[1] == '{' {
			// "${VAR}"
			for i := 2; i < len(val); i++ {
				if val[i] == '}' {
					return os.Getenv(val[2:i])
				}
			}
		}
		return os.Getenv(val[1:])
	}
	return val
}

// addrToID returns a short stable ID derived from an address.
func addrToID(addr string) string {
	// Simple: use last segment of address for readability.
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' || addr[i] == '.' {
			return addr[i+1:]
		}
	}
	return addr
}

// ConsulCatalog is a Catalog backed by HashiCorp Consul.
type ConsulCatalog struct {
	addr       string
	token      string
	datacenter string
	mu         sync.RWMutex
	// In-process TTL cache to avoid hammering Consul on every Find call.
	// For full production use, replace with github.com/hashicorp/consul/api.
	cache map[string][]Service
}

// Register implements Catalog using Consul's Agent.ServiceRegister.
func (c *ConsulCatalog) Register(_ context.Context, svc Service) error {
	// Real implementation would call consul.api.Agent().ServiceRegister().
	// Here we maintain an in-process cache as a stand-in.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = make(map[string][]Service)
	}
	c.cache[svc.Name] = append(c.cache[svc.Name], svc)
	return nil
}

// Deregister implements Catalog.
func (c *ConsulCatalog) Deregister(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, svcs := range c.cache {
		filtered := make([]Service, 0, len(svcs))
		for _, s := range svcs {
			if s.ID != id {
				filtered = append(filtered, s)
			}
		}
		c.cache[name] = filtered
	}
	return nil
}

// Find implements Catalog.
func (c *ConsulCatalog) Find(_ context.Context, name string, _ []string) ([]Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Service(nil), c.cache[name]...), nil
}

// Health implements Catalog.
func (c *ConsulCatalog) Health(_ context.Context, id string) error {
	// Real implementation: consul.api.Agent().UpdateTTL().
	return nil
}

// DNSCatalog is a Catalog that resolves services via DNS SRV records.
// It performs a lookup on every Find call (no caching).
type DNSCatalog struct {
	domain string
}

// Register is a no-op for DNSCatalog (DNS records are managed externally).
func (c *DNSCatalog) Register(_ context.Context, _ Service) error {
	return nil
}

// Deregister is a no-op for DNSCatalog.
func (c *DNSCatalog) Deregister(_ context.Context, _ string) error {
	return nil
}

// Find resolves a service by performing a DNS SRV lookup for
// "_<name>._tcp.<domain>" and "_<name>._udp.<domain>".
func (c *DNSCatalog) Find(ctx context.Context, name string, _ []string) ([]Service, error) {
	srvRecords, err := lookupSRV(ctx, "_"+name+"._tcp."+c.domain)
	if err != nil {
		// Fall back to UDP.
		srvRecords, err = lookupSRV(ctx, "_"+name+"._udp."+c.domain)
		if err != nil {
			return nil, &ServiceNotFoundError{Name: name}
		}
	}
	out := make([]Service, 0, len(srvRecords))
	for _, r := range srvRecords {
		out = append(out, Service{
			ID:      fmt.Sprintf("%s-%s-%d", name, r.Target, r.Port),
			Name:    name,
			Address: fmt.Sprintf("%s:%d", r.Target, r.Port),
		})
	}
	return out, nil
}

// Health is a no-op for DNSCatalog.
func (c *DNSCatalog) Health(_ context.Context, _ string) error { return nil }

// SRVRecord holds the result of a DNS SRV lookup.
type SRVRecord struct {
	Target   string
	Port     uint16
	Priority uint16
	Weight   uint16
}

// lookupSRV performs a DNS SRV lookup. Extracted as a variable for testability.
var lookupSRV = func(ctx context.Context, name string) ([]SRVRecord, error) {
	_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "", "", name)
	if err != nil {
		return nil, err
	}
	out := make([]SRVRecord, len(addrs))
	for i, a := range addrs {
		out[i] = SRVRecord{
			Target:   a.Target,
			Port:     a.Port,
			Priority: a.Priority,
			Weight:   a.Weight,
		}
	}
	return out, nil
}
