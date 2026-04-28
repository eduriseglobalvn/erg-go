// Package shared provides cross-cutting utilities for erg.ninja lib/ clients.
package shared

import (
	"context"
	"fmt"
	"sync"

	"erg.ninja/pkg/discovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Factory is a discovery-aware gRPC client factory.
// It registers a discovery resolver and provides preconfigured dial options
// so individual lib/ clients can use service discovery without boilerplate.
type Factory struct {
	catalog discovery.Catalog
	mu      sync.RWMutex
}

// NewFactory creates a Factory from a discovery Catalog.
// The Catalog is used to dynamically resolve service addresses via DNS-SRV
// or a centralized registry (Consul/Static). If catalog is nil, NewFactory
// returns a factory that uses direct address resolution.
func NewFactory(catalog discovery.Catalog) *Factory {
	if catalog != nil {
		grpcResolverRegistered.Do(func() {
			// We register per-name via discovery.NewResolver at dial time instead
			// of a global builder, so no global registration needed here.
		})
	}
	return &Factory{catalog: catalog}
}

var grpcResolverRegistered sync.Once

// BuildDialOptions returns gRPC dial options configured for discovery.
// If a catalog is available, it uses the discovery scheme with the provided
// service name as the target endpoint. Otherwise it uses direct address.
func (f *Factory) BuildDialOptions(serviceName string, opts ...grpc.DialOption) []grpc.DialOption {
	var base []grpc.DialOption

	if f.catalog != nil {
		// discovery:// scheme routes through discovery.Resolver
		// which queries the Catalog on a ticker for healthy instances.
		resolverBuilder := discovery.NewResolver(f.catalog, serviceName)
		base = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithResolvers(resolverBuilder),
			grpc.WithBlock(),
		}
	} else {
		base = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		}
	}

	out := make([]grpc.DialOption, 0, len(base)+len(opts))
	out = append(out, base...)
	out = append(out, opts...)
	return out
}

// Dial dials a gRPC service using discovery. target is the service name
// (used as the discovery lookup key) or a direct "host:port" address.
// Returns the connection and a CloseFunc.
func (f *Factory) Dial(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, func(), error) {
	dialOpts := f.BuildDialOptions(target, opts...)

	// Use discovery scheme if catalog is available, otherwise direct target.
	// gRPC treats "discovery:///service-name" as a resolver target (scheme://authority/endpoint).
	var dialTarget string
	if f.catalog != nil {
		dialTarget = "discovery:///" + target
	} else {
		dialTarget = target
	}

	conn, err := grpc.DialContext(ctx, dialTarget, dialOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("shared/factory: dial %q: %w", dialTarget, err)
	}
	return conn, func() { conn.Close() }, nil
}

// Catalog returns the underlying discovery Catalog, if any.
func (f *Factory) Catalog() discovery.Catalog {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.catalog
}

// WithServiceName is a convenience type to allow NewClientWithDiscovery
// to be called with named parameters.
type WithServiceName struct{ name string }

// WithDiscovery returns a service name hint for the factory.
// This is passed to NewClient factory methods to enable dynamic discovery.
func WithDiscovery(serviceName string) WithServiceName {
	return WithServiceName{name: serviceName}
}
