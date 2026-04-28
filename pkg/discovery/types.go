// Package discovery provides client-side service discovery for erg.ninja microservices.
// It supports Consul, DNS SRV, and a static in-memory catalog for development.
package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service represents a single discovered service instance.
type Service struct {
	ID       string            // unique instance ID, e.g. "crawler-7f8d6b4-xkq2p"
	Name     string            // logical service name, e.g. "crawler", "notification"
	Version  string            // API version, e.g. "v1", "v2"
	Address  string            // "host:port", e.g. "10.0.1.5:8083"
	Tags     []string          // arbitrary labels, e.g. ["grpc", "tenant=acme"]
	Metadata map[string]string // additional metadata: region, load, health state
	TTL      time.Time         // heartbeat expiry; zero = no expiry
}

// Catalog is the service discovery backend interface.
// Implementations: ConsulCatalog, DNSCatalog, StaticCatalog.
type Catalog interface {
	// Register advertises a service instance. Idempotent (updates if already exists).
	Register(ctx context.Context, svc Service) error
	// Deregister removes a service instance from the catalog.
	Deregister(ctx context.Context, id string) error
	// Find returns all healthy instances of the named service, filtered by optional tags.
	Find(ctx context.Context, name string, tags []string) ([]Service, error)
	// Health refreshes the TTL for a given instance ID (heartbeat).
	Health(ctx context.Context, id string) error
}

// StaticCatalog is a simple in-memory Catalog for local development and testing.
// It requires no external infrastructure.
type StaticCatalog struct {
	mu   sync.RWMutex
	svcs map[string][]Service // key = service name
}

// NewStaticCatalog creates a catalog pre-populated from the given map.
func NewStaticCatalog(svcs map[string][]Service) *StaticCatalog {
	if svcs == nil {
		svcs = make(map[string][]Service)
	}
	return &StaticCatalog{svcs: svcs}
}

// Register implements Catalog.
func (c *StaticCatalog) Register(_ context.Context, svc Service) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	name := svc.Name
	c.svcs[name] = append(c.svcs[name], svc)
	return nil
}

// Deregister implements Catalog.
func (c *StaticCatalog) Deregister(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, svcs := range c.svcs {
		filtered := make([]Service, 0, len(svcs))
		for _, s := range svcs {
			if s.ID != id {
				filtered = append(filtered, s)
			}
		}
		c.svcs[name] = filtered
	}
	return nil
}

// Find implements Catalog. Returns all registered instances matching name and tags.
// Tag matching uses AND semantics: a service must have ALL query tags to match.
func (c *StaticCatalog) Find(_ context.Context, name string, tags []string) ([]Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	svcs, ok := c.svcs[name]
	if !ok {
		return nil, nil
	}
	if len(tags) == 0 {
		return append([]Service(nil), svcs...), nil
	}
	out := make([]Service, 0, len(svcs))
	for _, s := range svcs {
		if !s.TTL.IsZero() && s.TTL.Before(time.Now()) {
			continue // expired
		}
		// Build a set of the service's own tags.
		sTagSet := make(map[string]struct{}, len(s.Tags))
		for _, t := range s.Tags {
			sTagSet[t] = struct{}{}
		}
		// AND: every query tag must be present in the service's tag set.
		hasAll := true
		for _, t := range tags {
			if _, found := sTagSet[t]; !found {
				hasAll = false
				break
			}
		}
		if hasAll {
			out = append(out, s)
		}
	}
	return out, nil
}

// Health implements Catalog (no-op for StaticCatalog).
func (c *StaticCatalog) Health(_ context.Context, _ string) error {
	return nil
}

// ServiceNotFoundError is returned when Find() finds no matching instances.
type ServiceNotFoundError struct {
	Name string
	Tags []string
}

func (e *ServiceNotFoundError) Error() string {
	return fmt.Sprintf("discovery: no instances found for service %q (tags=%v)", e.Name, e.Tags)
}
